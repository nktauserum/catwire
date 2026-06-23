package main

import (
	"crypto/ecdh"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/client/config"
	"github.com/nktauserum/catwire/common"
)

var nextSequenceNumber atomic.Uint64

type Client struct {
	incoming chan common.Packet
	outgoing chan []byte

	curve            ecdh.Curve
	clientPrivateKey *ecdh.PrivateKey
	clientPublicKey  *ecdh.PublicKey

	crypto    *common.Crypto
	peerIndex uint64
}

func (c *Client) Start() {
	for range 5 {
		p := common.Packet{
			Header: common.Header{
				PacketType: common.HANDSHAKE_INIT,
				PeerIndex:  0,
				Counter:    nextSequenceNumber.Load(),
			},
			Payload: c.clientPublicKey.Bytes(),
		}

		encHandshake := common.EncodePacket(p)

		c.outgoing <- encHandshake

		select {
		case resp := <-c.incoming:
			if resp.Header.PacketType != common.HANDSHAKE_INIT {
				return
			}

			var err error

			serverPub, err := c.curve.NewPublicKey(resp.Payload)
			if err != nil {
				log.Printf("error creating new public key: %v\n", err)
				return
			}

			secret, err := c.clientPrivateKey.ECDH(serverPub)
			if err != nil {
				log.Printf("error computing the secret: %v\n", err)
				return
			}

			log.Println("The shared secret was computed!")

			crypto, err := common.NewCrypto(secret)
			if err != nil {
				log.Printf("error creating crypto: %v\n", err)
				return
			}

			c.peerIndex = resp.Header.PeerIndex
			c.crypto = crypto

			return

		case <-time.After(4 * time.Second):
			continue
		}
	}

	for p := range c.incoming {
		log.Printf("Unknown packet with type %v\n", p.Header.PacketType)
	}
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to config")
	flag.Parse()

	if configPath == "" {
		fmt.Println("Please provide a relevant config. For more info see --help.")
		os.Exit(1)
	}

	config, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("error parsing config: %v\n", err)
	}

	c := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: "cw0",
		},
	}

	tun, err := water.New(c)
	if err != nil {
		log.Fatalln("error create tun: ", err)
	}
	defer tun.Close()

	serverIP, _, err := net.SplitHostPort(config.ServerAddr)
	if err != nil {
		log.Fatalf("Error parsing server address, check it: %v\n", err)
	}

	cmds := [][]string{
		{"ip", "link", "set", tun.Name(), "up"},
		{"ip", "addr", "add", config.PeerAddr + "/32", "dev", tun.Name()},
		{"ip", "link", "set", "dev", tun.Name(), "mtu", "1420"},
		{"ip", "route", "replace", "10.0.5.0/24", "dev", tun.Name()},

		{"ip", "route", "add", "default", "dev", tun.Name(), "table", "100"},
		{"ip", "rule", "add", "priority", "100", "to", serverIP, "lookup", "main"},
		{"ip", "rule", "add", "priority", "200", "lookup", "100"},
		{"ip", "rule", "add", "priority", "111", "to", "172.16.0.0/12", "lookup", "main"},
		{"ip", "rule", "add", "priority", "112", "to", "192.168.0.0/16", "lookup", "main"},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run %v: %v, output: %s", cmd, err, string(out))
		}
	}

	defer func() {
		cmds := [][]string{
			{"ip", "rule", "del", "priority", "112"},
			{"ip", "rule", "del", "priority", "111"},
			{"ip", "rule", "del", "priority", "200"},
			{"ip", "rule", "del", "priority", "100"},
		}

		for _, cmd := range cmds {
			out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
			if err != nil {
				log.Printf("Failed to run %v: %v, output: %s", cmd, err, string(out))
			}
		}
	}()

	log.Println("Successfully created TUN interface")

	curve := ecdh.X25519()
	privKeyBytes, err := base64.StdEncoding.DecodeString(config.PrivateKey)
	if err != nil {
		log.Fatalln("decode private key:", err)
	}

	clientPrivateKey, err := curve.NewPrivateKey(privKeyBytes)
	if err != nil {
		log.Fatalf("error generating client private key: %v\n", err)
	}
	clientPublicKey := clientPrivateKey.PublicKey()

	conn, err := net.Dial("udp", config.ServerAddr)
	if err != nil {
		log.Fatalln("error dialing to the server: ", err)
	}
	defer conn.Close()

	log.Printf("Dialing the connection to the server on %s\n", config.ServerAddr)

	incoming := make(chan common.Packet, 1024)
	outgoing := make(chan []byte, 1024)

	client := Client{

		outgoing: outgoing,
		incoming: incoming,

		curve:            curve,
		clientPrivateKey: clientPrivateKey,
		clientPublicKey:  clientPublicKey,

		crypto:    nil,
		peerIndex: 0,
	}

	// start send loop
	go client.sendUDP(conn)
	go client.listenUDP(conn, tun)
	go client.listenTUN(tun)

	go client.Start()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	defer signal.Stop(ch)

	<-ch
	println()
	log.Printf("Graceful shutdown\n")
}

func (c *Client) listenTUN(tun *water.Interface) {
	buf := make([]byte, 65535)

	for {
		n, err := tun.Read(buf)
		if err != nil {
			continue
		}

		if c.crypto == nil {
			continue
		}

		counter := nextSequenceNumber.Add(1) - 1

		encryptedData, err := c.crypto.Encrypt(buf[:n], counter)
		if err != nil {
			log.Printf("listenTUN: encrypt: %v\n", err)
			continue
		}

		p := common.Packet{
			Header: common.Header{
				PacketType: common.DATA,
				PeerIndex:  c.peerIndex,
				Counter:    counter,
			},
			Payload: encryptedData,
		}

		encodedPacket := common.EncodePacket(p)

		c.outgoing <- encodedPacket
	}
}

func (c *Client) sendUDP(conn net.Conn) {
	for packet := range c.outgoing {
		if _, err := conn.Write(packet); err != nil {
			log.Printf("write: %v", err)
			continue
		}
	}
}

func (c *Client) listenUDP(conn net.Conn, tun *water.Interface) {
	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			continue
		}

		p := common.DecodePacket(buf[:n])

		if p.Header.PacketType == common.DATA {
			if c.crypto == nil {
				continue
			}
			decryptedData, err := c.crypto.Decrypt(p.Payload, p.Header.Counter)
			if err != nil {
				log.Printf("listenUDP: decrypt: %v\n", err)
				continue
			}

			if _, err = tun.Write(decryptedData); err != nil {
				log.Printf("error writing to TUN: %v\n", err)
			}
			continue
		}

		c.incoming <- p
	}
}
