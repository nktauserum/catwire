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
	"sync"
	"time"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/client/config"
	"github.com/nktauserum/catwire/common"
	"github.com/nktauserum/catwire/common/routing"
	"github.com/nktauserum/catwire/common/session"
)

type Client struct {
	conn *net.UDPConn
	tun  *water.Interface

	curve            ecdh.Curve
	clientPrivateKey *ecdh.PrivateKey
	clientPublicKey  *ecdh.PublicKey

	serverSession *session.Session

	peerTable     routing.PeerTable
	incomingTable IncomingTable
}

type IncomingTable struct {
	table []chan Message

	mu sync.RWMutex
}

func NewIncomingTable() IncomingTable {
	return IncomingTable{
		table: make([]chan Message, 0, 8),
	}
}

func (t *IncomingTable) Send(idx uint64, msg Message) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if idx > uint64(len(t.table)) {
		return
	}

	t.table[idx] <- msg
}

func (t *IncomingTable) Get(idx uint64) (chan Message, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if idx > uint64(len(t.table)) {
		return nil, fmt.Errorf("channel not fount by idx %v", idx)
	}

	return t.table[idx], nil
}

func (t *IncomingTable) Create() (uint64, chan Message) {
	ch := make(chan Message, 1024)

	t.mu.Lock()
	defer t.mu.Unlock()

	idx := uint64(len(t.table))
	t.table = append(t.table, ch)

	return idx, ch
}

func (c *Client) Handshake(remoteAddr string) (*session.Session, chan Message, error) {
	addr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		return nil, nil, err
	}

	idx, ch := c.incomingTable.Create()

	p := common.Packet{
		Header: common.Header{
			PacketType: common.HANDSHAKE_INIT,
			PeerIndex:  idx,
			Counter:    0,
		},
		Payload: c.clientPublicKey.Bytes(),
	}
	encHandshake := common.EncodePacket(p)

	for range 5 {
		_, err = c.conn.WriteToUDP(encHandshake, addr)
		if err != nil {
			log.Printf("Error sending packet: %v\n", err)
			return nil, nil, err
		}

		select {
		case t := <-ch:
			resp := t.Packet
			if resp.Header.PacketType != common.HANDSHAKE_RESPONSE {
				continue
			}

			var err error
			serverPub, err := c.curve.NewPublicKey(resp.Payload)
			if err != nil {
				log.Printf("error creating new public key: %v\n", err)
				return nil, nil, err
			}

			secret, err := c.clientPrivateKey.ECDH(serverPub)
			if err != nil {
				log.Printf("error computing the secret: %v\n", err)
				return nil, nil, err
			}

			log.Printf("The shared secret for %v was computed!\n", remoteAddr)

			crypto, err := common.NewCrypto(secret)
			if err != nil {
				log.Printf("error creating crypto: %v\n", err)
				return nil, nil, err
			}

			s := session.NewSession(c.conn, addr)
			s.InitSession(resp.Header.PeerIndex, crypto, serverPub)

			return s, ch, nil

		case <-time.After(4 * time.Second):
			continue
		}
	}

	return nil, nil, fmt.Errorf("timeout: give up handshaking after five retries")
}

type Message struct {
	Packet     common.Packet
	ClientAddr *net.UDPAddr
}

func (c *Client) Start(serverAddr string) {
	s, ch, err := c.Handshake(serverAddr)
	if err != nil {
		log.Fatalf("Handshake error: %v\n", err)
	}

	c.StartPeerSession(s, ch)
}

func (c *Client) StartPeerSession(session *session.Session, ch chan Message) {
	for t := range ch {
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

		case common.DATA:
			decrypted, err := session.Incoming(p, nil)
			if err != nil {
				continue
			}

			if _, err = c.tun.Write(decrypted); err != nil {
				log.Printf("error writing to TUN: %v\n", err)
			}

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

var ErrNotConnected = fmt.Errorf("peer not connected")

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
					go c.Start(peer.RemoteAddress.String())
					return ErrNotConnected
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

func (c *Client) listenUDP() {
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

		log.Printf("Packet: %#v\n", p)

		if p.Header.PacketType == common.HANDSHAKE_INIT {
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
			idx, ch := c.incomingTable.Create()
			s.InitSession(p.Header.PeerIndex, crypto, publicKey)

			peer.Session = s

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

			go c.StartPeerSession(s, ch)
		}

		c.incomingTable.Send(p.Header.PeerIndex, Message{Packet: p, ClientAddr: clientAddr})
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

	client := Client{
		conn:     conn,
		tun: tun,

		curve:            curve,
		clientPrivateKey: clientPrivateKey,
		clientPublicKey:  clientPublicKey,

		peerTable:  routing.NewPeerTable(),
		incomingTable: NewIncomingTable(),
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
