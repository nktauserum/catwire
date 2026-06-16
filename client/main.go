package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"log"
	"net"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/common"
)

const (
	ipAddr    = "10.0.5.2"
	peer      = "10.0.5.1"
	serverUDP = "94.232.42.18:55635"
)

var nextSequenceNumber atomic.Uint64

type Client struct {
	serverPublicKey *ecdh.PublicKey
	secret          []byte

	incoming chan common.Packet
	outgoing chan []byte

	curve            ecdh.Curve
	clientPrivateKey *ecdh.PrivateKey
	clientPublicKey  *ecdh.PublicKey

	crypto *common.Crypto
}

func (c *Client) Start() {
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

	for i := range 3 {
		log.Printf("Handshake #%v\n", i)
		select {
		case resp := <-c.incoming:
			if resp.Header.PacketType != common.HANDSHAKE_INIT {
				return
			}

			var err error

			c.serverPublicKey, err = c.curve.NewPublicKey(resp.Payload)
			if err != nil {
				log.Printf("error creating new public key: %v\n", err)
				return
			}

			c.secret, err = c.clientPrivateKey.ECDH(c.serverPublicKey)
			if err != nil {
				log.Printf("error computing the secret: %v\n", err)
				return
			}

			log.Printf("secret: %v\n", c.secret)

			c.crypto, err = common.NewCrypto(c.secret)
			if err != nil {
				log.Printf("error creating crypto: %v\n", err)
				return
			}

			return

		case <-time.After(2 * time.Second):
			continue
		}
	}
}

func main() {
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

	cmds := [][]string{
		{"ip", "link", "set", tun.Name(), "up"},
		{"ip", "addr", "add", ipAddr + "/32", "peer", peer, "dev", tun.Name()},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run %v: %v, output: %s", cmd, err, string(out))
		}
	}

	log.Println("Successfully created TUN interface")

	curve := ecdh.X25519()
	clientPrivateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("error generating client private key: %v\n", err)
	}
	clientPublicKey := clientPrivateKey.PublicKey()

	conn, err := net.Dial("udp", serverUDP)
	if err != nil {
		log.Fatalln("error dialing to the server: ", err)
	}
	defer conn.Close()

	log.Printf("Dialing the connection to the server on %s\n", serverUDP)

	incoming := make(chan common.Packet)
	outgoing := make(chan []byte)

	client := Client{
		serverPublicKey: nil,
		secret:          nil,

		outgoing: outgoing,
		incoming: incoming,

		curve:            curve,
		clientPrivateKey: clientPrivateKey,
		clientPublicKey:  clientPublicKey,

		crypto: nil,
	}

	// start send loop
	go client.sendUDP(conn)
	go client.listenUDP(conn, tun)
	go client.listenTUN(tun)

	client.Start()

	select {}
}

func (c *Client) listenTUN(tun *water.Interface) {
	buf := make([]byte, 65535)

	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Printf("read: %v\n", err)
			continue
		}

		if c.crypto == nil {
			continue
		}

		log.Printf("Outgoing packet: to %v len(%v)\n", common.ExtractDestinationIP(buf[:n]), n)

		encryptedData, err := c.crypto.Encrypt(buf[:n])
		if err != nil {
			log.Printf("listenTUN: encrypt: %v\n", err)
			continue
		}

		p := common.Packet{
			Header: common.Header{
				PacketType: common.DATA,
				PeerIndex:  0,
				Counter:    nextSequenceNumber.Load(),
			},
			Payload: encryptedData,
		}

		encodedPacket := common.EncodePacket(p)
		nextSequenceNumber.Add(1)

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
			log.Printf("error reading from %s: %v\n", conn.RemoteAddr(), err)
			continue
		}

		p := common.DecodePacket(buf[:n])

		log.Printf("Incoming packet: len(%v)\n", n)

		if p.Header.PacketType == common.DATA {
			if c.crypto == nil {
				continue
			}
			decryptedData, err := c.crypto.Decrypt(p.Payload)
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
