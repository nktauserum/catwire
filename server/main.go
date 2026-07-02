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
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/common"
	"github.com/nktauserum/catwire/server/config"
)

const ipAddr = "10.0.5.1"                                 // as a server
const subnetMask = (0xFFFFFFFF << (32 - 24)) & 0xFFFFFFFF // 24 as CIDR notation (0xFFFFFF00)

var subnetAddr = getIPSubnet(ipAsInteger(ipAddr), subnetMask)

type Session struct {
	clientPublicKey *ecdh.PublicKey
	secret          []byte

	outgoing chan []byte

	crypto *common.Crypto

	remoteAddr atomic.Pointer[net.UDPAddr]
	conn       *net.UDPConn
	peerIndex  uint64
	Counter    atomic.Uint64

	IPLookupTable *PeerRouting
}

type PeerIndices struct {
	lookupTable []*Session
	mu          sync.Mutex
}

func (pi *PeerIndices) Load(peerIndex uint64) (*Session, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	p := peerIndex - 1

	if p >= uint64(len(pi.lookupTable)) {
		return nil, fmt.Errorf("no such peerIndex")
	}

	s := pi.lookupTable[p]

	if s == nil {
		return nil, fmt.Errorf("session equals nil")
	}

	return s, nil
}

func (pi *PeerIndices) Store(session *Session, key string) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	// compare already existing and incoming sessions using the encoded private key
	for i := range pi.lookupTable { // O(n) but acceptable for rare handshakes
		k := base64.StdEncoding.EncodeToString(
			pi.lookupTable[i].clientPublicKey.Bytes(),
		)

		if k == key {
			session.peerIndex = uint64(i + 1)
			pi.lookupTable[i] = session
			return
		}
	}

	// if it doesn't exist, create a new entry
	session.peerIndex = uint64(len(pi.lookupTable)) + 1
	pi.lookupTable = append(pi.lookupTable, session)
}

type PeerRouting struct {
	lookupTable map[uint32]*Session
	mu          sync.RWMutex
}

type Server struct {
	IndexLookupTable PeerIndices
	AllowedIPs       map[string]uint32
	IPLookupTable    PeerRouting

	curve            ecdh.Curve
	serverPrivateKey *ecdh.PrivateKey
	serverPublicKey  *ecdh.PublicKey

	outgoing chan []byte

	conn *net.UDPConn
}

type Task struct {
	Data       *[]byte
	ClientAddr *net.UDPAddr
}

func (s *Server) listenUDP() {
	buf := make([]byte, 65535)
	pool := sync.Pool{
		New: func() any {
			b := make([]byte, 65535)
			return &b
		},
	}
	ch := make(chan *Task, 1024)

	workersCount := 32

	for range workersCount {
		go func() {
			for t := range ch {
				p, err := common.DecodePacket(*t.Data)
				if err != nil {
					continue
				}

				if p.Header.PacketType == common.DATA && p.Header.PeerIndex != 0 {
					session, err := s.IndexLookupTable.Load(p.Header.PeerIndex)
					if err != nil {
						log.Printf("Error loading session with index %v: %v\n", p.Header.PeerIndex, err)
						continue
					}

					session.Incoming(p, t.ClientAddr)
					continue
				}

				// handshake
				if p.Header.PacketType != common.HANDSHAKE_INIT {
					continue
				}

				key := base64.StdEncoding.EncodeToString(p.Payload)
				clientIP, exists := s.AllowedIPs[key]
				if exists {
					session := &Session{
						outgoing: s.outgoing,
						conn:     s.conn,
					}

					session.remoteAddr.Store(t.ClientAddr)

					var err error
					session.clientPublicKey, err = s.curve.NewPublicKey(p.Payload)
					if err != nil {
						log.Printf("error creating new public key: %v\n", err)
						continue
					}

					secret, err := s.serverPrivateKey.ECDH(session.clientPublicKey)
					if err != nil {
						log.Printf("error computing the secret: %v\n", err)
						continue
					}

					log.Printf("The shared secret for %v was computed!\n", t.ClientAddr)

					session.crypto, err = common.NewCrypto(secret)
					if err != nil {
						log.Printf("error creating crypto: %v\n", err)
						continue
					}

					s.IndexLookupTable.Store(session, key)

					s.IPLookupTable.mu.Lock()
					s.IPLookupTable.lookupTable[clientIP] = session
					s.IPLookupTable.mu.Unlock()

					session.IPLookupTable = &s.IPLookupTable

					resp := common.Packet{
						Header: common.Header{
							PacketType: common.HANDSHAKE_INIT,
							PeerIndex:  session.peerIndex,
							Counter:    session.Counter.Add(1) - 1,
						},
						Payload: s.serverPublicKey.Bytes(),
					}

					enc := common.EncodePacket(resp)

					session.send(enc) // вызываем внутреннюю функцию Session для отправки байтов сразу в UDP
					pool.Put(t.Data)
				}
			}
		}()
	}

	for {
		n, clientAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("read: ", err)
			continue
		}

		data := pool.Get().(*[]byte)
		*data = (*data)[:n]
		copy(*data, buf[:n])

		ch <- &Task{Data: data, ClientAddr: clientAddr}
	}
}

func (s *Session) send(data []byte) {
	if clientAddr := s.remoteAddr.Load(); clientAddr != nil {
		if _, err := s.conn.WriteToUDP(data, clientAddr); err != nil {
			log.Println("write: ", err)
		}
	}
}

func getIPSubnet(ip uint32, mask uint32) uint32 {
	return ip & mask
}

func IPInLocalSubnet(ip uint32) bool {
	return getIPSubnet(ip, subnetMask) == subnetAddr
}

func (s *Session) Incoming(p common.Packet, remoteAddr *net.UDPAddr) {
	decrypted, err := s.crypto.Decrypt(p.Payload, p.Header.Counter)
	if err != nil {
		return
	}

	s.remoteAddr.Store(remoteAddr)

	destIP := common.ExtractDestinationIP(decrypted)
	if IPInLocalSubnet(destIP) && destIP != ipAsInteger(ipAddr) { // only if destIP owned by our virtual network and it isn't server's address (because it doesn't exist in IPLookupTable)
		s.IPLookupTable.mu.RLock()
		session := s.IPLookupTable.lookupTable[destIP]
		s.IPLookupTable.mu.RUnlock()

		if session == nil {
			log.Printf("Send to a non-established connection\n")
			return
		}

		session.Outgoing(decrypted)

		return
	}

	s.outgoing <- decrypted // to TUN
}

func (s *Session) Outgoing(data []byte) {
	counter := s.Counter.Add(1) - 1

	encrypted, err := s.crypto.Encrypt(data, counter)
	if err != nil {
		log.Printf("Error encrypt packet: %v\n", err)
		return
	}

	p := common.Packet{
		Header: common.Header{
			PacketType: common.DATA,
			PeerIndex:  s.peerIndex,
			Counter:    counter,
		},
		Payload: encrypted,
	}

	encoded := common.EncodePacket(p)
	s.send(encoded) // directly to UDP
}

func (s *Server) listenTUN(tun *water.Interface) {
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
				destIP := common.ExtractDestinationIP(*data)

				s.IPLookupTable.mu.RLock()
				session := s.IPLookupTable.lookupTable[destIP]
				s.IPLookupTable.mu.RUnlock()

				if session == nil {
					continue
				}

				session.Outgoing(*data)
				pool.Put(data)
			}
		}()
	}

	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Println("write: ", err)
			continue
		}

		data := pool.Get().(*[]byte)
		*data = (*data)[:n]
		copy(*data, buf[:n])

		ch <- data
	}
}

func sendTUN(tun *water.Interface, outgoing chan []byte) {
	for data := range outgoing {
		if _, err := tun.Write(data); err != nil {
			log.Println("sendTUN: ", err)
		}
	}
}

func ipAsInteger(s string) uint32 {
	ip := net.ParseIP(s).To4()

	return binary.BigEndian.Uint32(ip)
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

	cmds := [][]string{
		{"ip", "link", "set", tun.Name(), "up"},
		{"ip", "addr", "add", ipAddr + "/24", "dev", tun.Name()},
		{"ip", "link", "set", "dev", tun.Name(), "mtu", "1420"},
		{"iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE"},
		{"iptables", "-A", "FORWARD", "-i", tun.Name(), "-j", "ACCEPT"},
		{"iptables", "-A", "FORWARD", "-o", tun.Name(), "-j", "ACCEPT"},
		{"sysctl", "-w", "net.ipv4.ip_forward=1"},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run %v: %v, output: %s", cmd, err, string(out))
		}
	}

	// init crypto
	curve := ecdh.X25519()
	privKeyBytes, err := base64.StdEncoding.DecodeString(config.PrivateKey)
	if err != nil {
		log.Fatalln("decode private key:", err)
	}

	serverPrivateKey, err := curve.NewPrivateKey(privKeyBytes)
	if err != nil {
		log.Fatalf("error generating client private key: %v\n", err)
	}
	serverPublicKey := serverPrivateKey.PublicKey()

	addr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(config.ListenPort))
	if err != nil {
		log.Fatalf("error resolving udp addr: %v\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	outgoing := make(chan []byte, 1024)

	allowedIPs := make(map[string]uint32)
	for key, ip := range config.AllowedIPs {
		allowedIPs[key] = ipAsInteger(ip)
	}

	s := Server{
		conn:     conn,
		outgoing: outgoing,

		curve:            curve,
		serverPrivateKey: serverPrivateKey,
		serverPublicKey:  serverPublicKey,

		IPLookupTable: PeerRouting{
			lookupTable: make(map[uint32]*Session),
		},
		IndexLookupTable: PeerIndices{
			lookupTable: make([]*Session, 0, len(allowedIPs)),
		},
		AllowedIPs: allowedIPs,
	}

	go sendTUN(tun, outgoing)
	go s.listenTUN(tun)

	log.Printf("Listening on :%d\n", config.ListenPort)

	s.listenUDP()
}
