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
	"sync"
	"sync/atomic"
	"time"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/client/config"
	"github.com/nktauserum/catwire/common"
	"github.com/nktauserum/catwire/common/session"
)

type Client struct {
	conn *net.UDPConn
	tun  *water.Interface

	curve            ecdh.Curve
	clientPrivateKey *ecdh.PrivateKey
	clientPublicKey  *ecdh.PublicKey

	serverSession *session.Session
	lastPacket    atomic.Uint32
	incoming      chan common.Packet
}

func (c *Client) incomingCallback(payload []byte) {
	if _, err := c.tun.Write(payload); err != nil {
		log.Println("incomingCallback: tun.Write: ", err)
	}
}

func (c *Client) outgoingCallback(data []byte, clientAddr *net.UDPAddr) {
	if _, err := c.conn.WriteToUDP(data, clientAddr); err != nil {
		log.Println("write: ", err)
	}
}

func (c *Client) Handshake(remoteAddr string) error {
	addr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return err
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

	for range 5 {
		_, err = c.conn.WriteToUDP(encHandshake, addr)
		if err != nil {
			log.Printf("Error sending packet: %v\n", err)
			return err
		}

		select {
		case resp := <-c.incoming:
			if resp.Header.PacketType != common.HANDSHAKE_INIT {
				continue
			}

			var err error
			serverPub, err := c.curve.NewPublicKey(resp.Payload)
			if err != nil {
				log.Printf("error creating new public key: %v\n", err)
				return err
			}

			secret, err := c.clientPrivateKey.ECDH(serverPub)
			if err != nil {
				log.Printf("error computing the secret: %v\n", err)
				return err
			}

			log.Printf("The shared secret for %v was computed!\n", remoteAddr)

			aesGCM, err := common.SetupEncryption(secret)
			if err != nil {
				log.Printf("error setting encryption up: %v\n", err)
				continue
			}

			s := session.NewSession(c.incomingCallback, c.outgoingCallback, addr)
			s.InitSession(resp.Header.PeerIndex, aesGCM, serverPub)

			c.serverSession = s

			return nil

		case <-time.After(4 * time.Second):
			continue
		}
	}

	return fmt.Errorf("timeout: give up handshaking after five retries")
}

func (c *Client) Start(serverAddr string) {
	err := c.Handshake(serverAddr)
	if err != nil {
		log.Fatalf("Handshake error: %v\n", err)
	}

	select {}
}

func (c *Client) listenTUN(tun *water.Interface) {
	buf := make([]byte, 65535)
	pool := sync.Pool{
		New: func() any {
			b := make([]byte, 65535)
			return &b
		},
	}
	ch := make(chan *[]byte, 1024)

	workersCount := 32
	for range workersCount {
		go func() {
			for p := range ch {
				if c.serverSession == nil {
					pool.Put(p)
					continue
				}

				c.serverSession.Outgoing(*p)
				pool.Put(p)
			}
		}()
	}

	for {
		n, err := tun.Read(buf)
		if err != nil {
			continue
		}

		data := pool.Get().(*[]byte)
		*data = (*data)[:n]
		copy(*data, buf[:n])

		ch <- data
	}
}

func (c *Client) listenUDP() {
	buf := make([]byte, 65535)
	pool := sync.Pool{
		New: func() any {
			b := make([]byte, 65535)
			return &b
		},
	}
	ch := make(chan *[]byte, 1024)


	workersCount := 32
	for range workersCount {
		go func() {
			for data := range ch {
				p, err := common.DecodePacket(*data)
				if err != nil {
					pool.Put(data)
					continue
				}

				if p.Header.PacketType == common.DATA {
					if c.serverSession == nil {
						pool.Put(data)
						continue
					}

					if !c.serverSession.Initialized() {
						pool.Put(data)
						continue
					}

					c.serverSession.Incoming(p, nil)
					pool.Put(data)
					continue
				}

				c.incoming <- p // pool leak (perhaps, acceptable?)
			}
		}()
	}

	for {
		n, _, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		data := pool.Get().(*[]byte)
		*data = (*data)[:n]
		copy(*data, buf[:n])

		ch <- data
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

	client := Client{
		conn: conn,
		tun:  tun,

		curve:            curve,
		clientPrivateKey: clientPrivateKey,
		clientPublicKey:  clientPublicKey,

		incoming: incoming,
	}

	// start send loop
	go client.listenUDP()
	go client.listenTUN(tun)

	go client.Start(config.ServerAddr)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	defer signal.Stop(ch)

	<-ch
	println()
	log.Printf("Graceful shutdown\n")
}
