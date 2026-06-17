package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"os/exec"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/common"
)

const ipAddr = "10.0.5.1"
const peer = "10.0.5.2"

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
	next        atomic.Uint64
}

func (pi *PeerIndices) Next() uint64 {
	n := pi.next.Load()
	pi.next.Add(1)
	return n
}

func (pi *PeerIndices) Add(peerIndex uint64, session *Session) {
	pi.mu.Lock()
	pi.lookupTable[peerIndex] = session
	pi.mu.Unlock()
	pi.next.Add(1)
}

func (pi *PeerIndices) Load(peerIndex uint64) (*Session, error) {
	if peerIndex >= pi.next.Load() {
		return nil, fmt.Errorf("no such peerIndex")
	}

	pi.mu.Lock()
	s := pi.lookupTable[peerIndex]
	pi.mu.Unlock()

	return s, nil
}

type PeerRouting struct {
	lookupTable map[string]*Session
	mu          sync.RWMutex
}

type Server struct {
	IndexLookupTable PeerIndices
	AllowedIPs       []string
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

		log.Printf("Incoming packet: from %v len(%v)\n", clientAddr, n)

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
			if slices.Contains(s.AllowedIPs, string(p.Payload)) {
				session := &Session{
					outgoing: s.outgoing,
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

				log.Printf("The shared secret was computed!\n")

				session.crypto, err = common.NewCrypto(secret)
				if err != nil {
					log.Printf("error creating crypto: %v\n", err)
					return
				}

				session.peerIndex = s.IndexLookupTable.Next()
				s.IndexLookupTable.Add(session.peerIndex, session)

				resp := common.Packet{
					Header: common.Header{
						PacketType: common.HANDSHAKE_INIT,
						PeerIndex:  session.peerIndex,
						Counter:    nextSequenceNumber.Load(),
					},
					Payload: s.serverPublicKey.Bytes(),
				}

				enc := common.EncodePacket(resp)

				session.send(enc) // вызываем внутреннюю функцию Session для отправки байтов сразу в UDP
			}
		} ()
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
	decrypted, err := s.crypto.Decrypt(p.Payload)
	if err != nil {
		log.Printf("Error decrypt packet from %v len(%v): %v\n", remoteAddr.String(), len(p.Payload), err)
		return
	}

	s.remoteAddr.Store(remoteAddr)

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
		log.Printf("listenTUN: to %v len(%v)\n", destIP, n)
		
		s.IPLookupTable.mu.RLock()
		session := s.IPLookupTable.lookupTable[destIP]
		s.IPLookupTable.mu.RUnlock()

		session.Outgoing(buf[:n])
	}
}

func sendTUN(tun *water.Interface, outgoing chan []byte) {
	for data := range outgoing {
		if _, err := tun.Write(data); err != nil {
			log.Println("read: ", err)
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

	cmds := [][]string{
		{"ip", "link", "set", tun.Name(), "up"},
		{"ip", "addr", "add", ipAddr + "/32", "peer", peer, "dev", tun.Name()},
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

	allowedIPs := []string {"pubkey1", "pubkey2"}

	s := Server {
		conn: conn,
		outgoing: outgoing,

		curve:            curve,
		serverPrivateKey: serverPrivateKey,
		serverPublicKey:  serverPublicKey,

		IPLookupTable: PeerRouting {
			lookupTable: make(map[string]*Session),
		},
		IndexLookupTable: PeerIndices {
			lookupTable: make([]*Session, 0, len(allowedIPs)),
		},
		AllowedIPs: allowedIPs,
	}

	go sendTUN(tun, outgoing)
	go s.listenTUN(tun)

	log.Printf("Listening on :55635\n")

	s.listenUDP()
}
