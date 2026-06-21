package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/common"
)

const ipAddr = "10.0.5.1" // as a server

var nextSequenceNumber atomic.Uint64

type Session struct {
	clientPublicKey *ecdh.PublicKey
	secret          []byte

	outgoing chan []byte

	crypto *common.Crypto

	remoteAddr atomic.Pointer[net.UDPAddr]
	conn       *net.UDPConn
	peerIndex  uint64
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

func (s *Server) listenUDP() {
	buf := make([]byte, 65535)
	for {
		n, clientAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("read: ", err)
			continue
		}

		p := common.DecodePacket(buf[:n])

		if p.Header.PacketType == common.DATA && p.Header.PeerIndex != 0 {
			session, err := s.IndexLookupTable.Load(p.Header.PeerIndex)
			if err != nil {
				log.Printf("Error loading session with index %v: %v\n", p.Header.PeerIndex, err)
				continue
			}

			go session.Incoming(p, clientAddr)

			continue
		}

		// handshake
		go func() {
			if p.Header.PacketType != common.HANDSHAKE_INIT {
				return
			}

			key := base64.StdEncoding.EncodeToString(p.Payload)
			clientIP, exists := s.AllowedIPs[key]
			if exists {
				session := &Session{
					outgoing: s.outgoing,
					conn:     s.conn,
				}

				session.remoteAddr.Store(clientAddr)

				var err error
				session.clientPublicKey, err = s.curve.NewPublicKey(p.Payload)
				if err != nil {
					log.Printf("error creating new public key: %v\n", err)
					return
				}

				secret, err := s.serverPrivateKey.ECDH(session.clientPublicKey)
				if err != nil {
					log.Printf("error computing the secret: %v\n", err)
					return
				}

				log.Printf("The shared secret for %v was computed!\n", clientAddr)

				session.crypto, err = common.NewCrypto(secret)
				if err != nil {
					log.Printf("error creating crypto: %v\n", err)
					return
				}

				s.IndexLookupTable.mu.Lock()
				session.peerIndex = uint64(len(s.IndexLookupTable.lookupTable)) + 1
				s.IndexLookupTable.lookupTable = append(s.IndexLookupTable.lookupTable, session)
				s.IndexLookupTable.mu.Unlock()

				s.IPLookupTable.mu.Lock()
				s.IPLookupTable.lookupTable[clientIP] = session
				s.IPLookupTable.mu.Unlock()

				resp := common.Packet{
					Header: common.Header{
						PacketType: common.HANDSHAKE_INIT,
						PeerIndex:  session.peerIndex,
						Counter:    nextSequenceNumber.Load(),
					},
					Payload: s.serverPublicKey.Bytes(),
				}
				nextSequenceNumber.Add(1)

				enc := common.EncodePacket(resp)

				session.send(enc) // вызываем внутреннюю функцию Session для отправки байтов сразу в UDP
			}
		}()
	}
}

func (s *Session) send(data []byte) {
	if clientAddr := s.remoteAddr.Load(); clientAddr != nil {
		if _, err := s.conn.WriteToUDP(data, clientAddr); err != nil {
			log.Println("write: ", err)
		}
	}
}

func (s *Session) Incoming(p common.Packet, remoteAddr *net.UDPAddr) {
	if p.Header.PeerIndex != s.peerIndex {
		log.Printf("Invalid peerIndex: expected %v, got %v\n", s.peerIndex, p.Header.PeerIndex)
		return
	}

	decrypted, err := s.crypto.Decrypt(p.Payload)
	if err != nil {
		return
	}

	go s.remoteAddr.Store(remoteAddr)

	s.outgoing <- decrypted // to TUN
}

func (s *Session) Outgoing(data []byte) {
	encrypted, err := s.crypto.Encrypt(data)
	if err != nil {
		log.Printf("Error encrypt packet: %v\n", err)
		return
	}

	p := common.Packet{
		Header: common.Header{
			PacketType: common.DATA,
			PeerIndex:  s.peerIndex,
			Counter:    nextSequenceNumber.Load(),
		},
		Payload: encrypted,
	}
	nextSequenceNumber.Add(1)

	encoded := common.EncodePacket(p)
	s.send(encoded) // directly to UDP
}

func (s *Server) listenTUN(tun *water.Interface) {
	buf := make([]byte, 65535)

	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Println("write: ", err)
			continue
		}

		destIP := common.ExtractDestinationIP(buf[:n])

		s.IPLookupTable.mu.RLock()
		session := s.IPLookupTable.lookupTable[destIP]
		s.IPLookupTable.mu.RUnlock()

		if session == nil {
			continue
		}

		session.Outgoing(buf[:n])
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
	serverPrivateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("error generating server private key: %v\n", err)
	}
	serverPublicKey := serverPrivateKey.PublicKey()

	addr, err := net.ResolveUDPAddr("udp", ":55635")
	if err != nil {
		log.Fatalf("error resolving udp addr: %v\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	outgoing := make(chan []byte)

	allowedIPs := map[string]uint32{
		"QdYN9vbo7o0kcquz5GltP+ZUUb7YMgmgngAQtkNbmRM=": ipAsInteger("10.0.5.2"),
		"HDEVOmoAhSHHrYQB8wtAAduzvF4yOS91ST1TZ3i2Z04=": ipAsInteger("10.0.5.3"),
		"w0EyAFT3/9wwSG5RVcuyPG+GB1wcdoRLyK9KmGHU0h0=": ipAsInteger("10.0.5.4"),
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

	log.Printf("Listening on :55635\n")

	s.listenUDP()
}
