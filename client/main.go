package main

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/client/config"
	"github.com/nktauserum/catwire/common"
	"github.com/nktauserum/catwire/common/routing"
	"github.com/nktauserum/catwire/common/session"
)

type Client struct {
	incoming chan common.Packet
	outgoing chan []byte
	conn     *net.UDPConn

	curve            ecdh.Curve
	clientPrivateKey *ecdh.PrivateKey
	clientPublicKey  *ecdh.PublicKey

	serverSession *session.Session

	addressTable routing.AddressTable
	indexTable   routing.IndexTable
}

func (c *Client) Handshake(remoteAddr string) (*session.Session, error) {
	addr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return nil, err
	}

	p := common.Packet{
		Header: common.Header{
			PacketType: common.HANDSHAKE_INIT,
			PeerIndex:  0,
			Counter:    0,
		},
		Payload: c.clientPublicKey.Bytes(),
	}
	encHandshake := common.EncodePacket(p)

	_, err = c.conn.WriteToUDP(encHandshake, addr)
	if err != nil {
		log.Printf("Error sending packet: %v\n", err)
		return nil, err
	}

	select {
	case resp := <-c.incoming:
		if resp.Header.PacketType != common.HANDSHAKE_INIT {
			return nil, err
		}

		var err error
		serverPub, err := c.curve.NewPublicKey(resp.Payload)
		if err != nil {
			log.Printf("error creating new public key: %v\n", err)
			return nil, err
		}

		secret, err := c.clientPrivateKey.ECDH(serverPub)
		if err != nil {
			log.Printf("error computing the secret: %v\n", err)
			return nil, err
		}

		log.Printf("The shared secret for %v was computed!\n", remoteAddr)

		crypto, err := common.NewCrypto(secret)
		if err != nil {
			log.Printf("error creating crypto: %v\n", err)
			return nil, err
		}

		s := session.NewSession(c.conn, addr)
		s.InitSession(resp.Header.PeerIndex, crypto, serverPub)

		return s, nil

	case <-time.After(4 * time.Second):
		return nil, fmt.Errorf("Timeout.")
	}
}

func (c *Client) Start(serverAddr string) {
	s, err := c.Handshake(serverAddr)
	if err != nil {
		log.Fatalf("Handshake error: %v\n", err)
	}

	c.serverSession = s

	for p := range c.incoming {
		switch p.Header.PacketType {
		case common.DISCOVER:
			payload, err := c.serverSession.Incoming(p, nil)
			if err != nil {
				log.Printf("Error decrypting discover payload: %v\n", err)
				continue
			}

			for offset := range len(payload) / 42 { // the entries count
				privateAddr := payload[offset:]
				publicAddr := payload[offset+5 : offset+9]
				port := binary.BigEndian.Uint16(payload[offset+10 : offset+12])

				var publicKey [32]byte
				copy(publicKey[:], payload[offset+13:offset+42])

				log.Printf("Entry #%v: %v %v:%v %v\n", offset+1, net.IP(privateAddr).String(), net.IP(publicAddr).String(), port, base64.StdEncoding.EncodeToString(publicKey[:]))
			}
		default:
			log.Printf("Unknown packet with type %v\n", p.Header.PacketType)
		}
	}
}

func (c *Client) listenTUN(tun *water.Interface) {
	buf := make([]byte, 65535)

	for {
		n, err := tun.Read(buf)
		if err != nil {
			continue
		}

		if c.serverSession == nil {
			continue
		}

		c.serverSession.Outgoing(buf[:n])
	}
}

func (c *Client) listenUDP(conn net.Conn, tun *water.Interface) {
	buf := make([]byte, 65535)

	for {
		n, err := conn.Read(buf)
		if err != nil {
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		p, err := common.DecodePacket(data)
		if err != nil {
			continue
		}

		if p.Header.PacketType == common.DATA {
			if c.serverSession == nil {
				continue
			}

			decrypted, err := c.serverSession.Incoming(p, nil)
			if err != nil {
				continue
			}

			if _, err = tun.Write(decrypted); err != nil {
				log.Printf("error writing to TUN: %v\n", err)
			}
			continue
		}

		c.incoming <- p
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
	}

	if config.ForwardAll {
		rules := [][]string{
			{"ip", "route", "add", "default", "dev", tun.Name(), "table", "100"},
			{"ip", "rule", "add", "priority", "100", "to", serverIP, "lookup", "main"},
			{"ip", "rule", "add", "priority", "200", "lookup", "100"},
			{"ip", "rule", "add", "priority", "111", "to", "172.16.0.0/12", "lookup", "main"},
			{"ip", "rule", "add", "priority", "112", "to", "192.168.0.0/16", "lookup", "main"},
		}

		cmds = append(cmds, rules...)
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run %v: %v, output: %s", cmd, err, string(out))
		}
	}

	defer func() {
		if !config.ForwardAll {
			return
		}

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

	addr, err := net.ResolveUDPAddr("udp", ":8245") //TODO: dehardcode the port
	if err != nil {
		log.Fatalf("error resolving udp addr: %v\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	log.Println("Listening on :8245")

	incoming := make(chan common.Packet, 1024)
	outgoing := make(chan []byte, 1024)

	client := Client{
		conn:     conn,
		outgoing: outgoing,
		incoming: incoming,

		curve:            curve,
		clientPrivateKey: clientPrivateKey,
		clientPublicKey:  clientPublicKey,

		addressTable: routing.NewAddressTable(),
		indexTable:   routing.NewIndexTable(1),
	}

	// start send loop
	go client.listenUDP(conn, tun)
	go client.listenTUN(tun)

	go client.Start(config.ServerAddr)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	defer signal.Stop(ch)

	<-ch
	println()
	log.Printf("Graceful shutdown\n")
}
