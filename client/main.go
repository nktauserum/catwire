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
	incoming chan Message
	outgoing chan []byte
	conn     *net.UDPConn

	curve            ecdh.Curve
	clientPrivateKey *ecdh.PrivateKey
	clientPublicKey  *ecdh.PublicKey

	serverSession *session.Session

	peerTable  routing.PeerTable
	indexTable routing.IndexTable
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
	case t := <-c.incoming:
		resp := t.Packet
		if resp.Header.PacketType != common.HANDSHAKE_RESPONSE {
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

type Message struct {
	Packet     common.Packet
	ClientAddr *net.UDPAddr
}

func (c *Client) Start(serverAddr string) {
	s, err := c.Handshake(serverAddr)
	if err != nil {
		log.Fatalf("Handshake error: %v\n", err)
	}

	c.serverSession = s

	for t := range c.incoming {
		p := t.Packet
		switch p.Header.PacketType {
		case common.DISCOVER:
			payload, err := c.serverSession.Incoming(p, nil)
			if err != nil {
				log.Printf("Error decrypting discover payload: %v\n", err)
				continue
			}

			if len(payload) < 42 {
				log.Printf("Malformed DISCOVER packet. Reason: too small (%v bytes)\n", len(payload))
				continue
			}

			for i := range len(payload) / 42 { // the entries count
				offset := i * 42
				privateAddr := payload[offset : offset+4]
				publicAddr := payload[offset+4 : offset+8]
				port := binary.BigEndian.Uint16(payload[offset+8 : offset+10])

				var publicKey [32]byte
				copy(publicKey[:], payload[offset+10:offset+42])

				log.Printf("Entry #%v: %v %v:%v %v\n", i, net.IP(privateAddr).String(), net.IP(publicAddr).String(), port, base64.StdEncoding.EncodeToString(publicKey[:]))

				addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", net.IP(publicAddr).String(), port))
				if err != nil {
					continue
				}

				pubKey, err := c.curve.NewPublicKey(publicKey[:])
				if err != nil {
					log.Printf("error creating new public key: %v\n", err)
					continue
				}

				c.peerTable.Add(
					binary.BigEndian.Uint32(privateAddr),
					addr,
					pubKey,
				)
			}

		case common.HANDSHAKE_INIT:
			key := base64.StdEncoding.EncodeToString(p.Payload)
			publicKey, err := c.curve.NewPublicKey(p.Payload)
			if err != nil {
				continue
			}

			_, peer, err := c.peerTable.Find(publicKey)
			if err != nil {
				continue // consider adding smth like ERROR packet return
			}

			secret, err := c.clientPrivateKey.ECDH(publicKey)
			if err != nil {
				continue
			}

			crypto, err := common.NewCrypto(secret)
			if err != nil {
				log.Printf("error creating crypto: %v\n", err)
				continue
			}

			s := session.NewSession(c.conn, peer.RemoteAddress)
			idx := c.indexTable.Store(key, s)
			s.InitSession(idx, crypto, publicKey)

			log.Printf("The connection with %v is successfully created!\n", peer.RemoteAddress.String())

			resp := common.Packet{
				Header: common.Header{
					PacketType: common.HANDSHAKE_RESPONSE,
					PeerIndex:  idx,
					Counter:    s.Counter.Add(1) - 1,
				},
				Payload: c.clientPublicKey.Bytes(),
			}

			enc := common.EncodePacket(resp)

			s.Send(enc) // вызываем внутреннюю функцию Session для отправки байтов сразу в UDP

		default:
			log.Printf("Unknown packet with type %v\n", p.Header.PacketType)
		}
	}
}

const ipAddr = "10.0.5.1"
const subnetMask = (0xFFFFFFFF << (32 - 24)) & 0xFFFFFFFF // 24 as CIDR notation (0xFFFFFF00)
var subnetAddr = getIPSubnet(common.IPAsInteger(ipAddr), subnetMask)

func getIPSubnet(ip uint32, mask uint32) uint32 {
	return ip & mask
}

func IPInLocalSubnet(ip uint32) bool {
	return getIPSubnet(ip, subnetMask) == subnetAddr
}

func (c *Client) listenTUN(tun *water.Interface) {
	buf := make([]byte, 65535)

	for {
		n, err := tun.Read(buf)
		if err != nil {
			continue
		}

		destIP := common.ExtractDestinationIP(buf[:n])
		if IPInLocalSubnet(destIP) && destIP != common.IPAsInteger(ipAddr) { // only if destIP owned by our virtual network and it isn't server's address (because it doesn't exist in IPLookupTable)
			err := func() error {
				peer, err := c.peerTable.Get(destIP)
				if err != nil {
					return err
				}

				if !peer.Connected() {
					// handshake
					return fmt.Errorf("Peer not connected")
				}

				peer.Session.Outgoing(buf[:n])
				log.Printf("Successfully sent to %v\n", peer.RemoteAddress.String())
				return nil
			}()
			if err == nil {
				continue
			}
			log.Printf("Error sending in local subnet: %v\n", err)
		}

		if c.serverSession == nil {
			continue
		}

		c.serverSession.Outgoing(buf[:n])
	}
}

func (c *Client) listenUDP(tun *water.Interface) {
	buf := make([]byte, 65535)

	for {
		n, clientAddr, err := c.conn.ReadFromUDP(buf)
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

		c.incoming <- Message{Packet: p, ClientAddr: clientAddr}
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

	incoming := make(chan Message, 1024)
	outgoing := make(chan []byte, 1024)

	client := Client{
		conn:     conn,
		outgoing: outgoing,
		incoming: incoming,

		curve:            curve,
		clientPrivateKey: clientPrivateKey,
		clientPublicKey:  clientPublicKey,

		peerTable:  routing.NewPeerTable(),
		indexTable: routing.NewIndexTable(1),
	}

	// start send loop
	go client.listenUDP(tun)
	go client.listenTUN(tun)

	go client.Start(config.ServerAddr)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	defer signal.Stop(ch)

	<-ch
	println()
	log.Printf("Graceful shutdown\n")
}
