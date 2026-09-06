package session

import (
	"crypto/ecdh"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"crypto/cipher"

	"github.com/nktauserum/catwire/common"
)

type Session struct {
	secret  []byte
	aesGCM cipher.AEAD
	Counter atomic.Uint64

	remoteAddr atomic.Pointer[net.UDPAddr]
	conn       *net.UDPConn
	PublicKey  *ecdh.PublicKey

	PeerIndex uint64

	mu sync.RWMutex
}

func NewSession(
	conn *net.UDPConn,
	addr *net.UDPAddr,
) *Session {
	s := &Session{
		conn: conn,
	}

	s.remoteAddr.Store(addr)
	return s
}

func (s *Session) InitSession(
	idx uint64,
	aesGCM cipher.AEAD,
	publicKey *ecdh.PublicKey,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.PeerIndex = idx
	s.PublicKey = publicKey
	s.aesGCM = aesGCM 
}

func (s *Session) Initialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.aesGCM != nil
}

func (s *Session) Send(data []byte) {
	if clientAddr := s.remoteAddr.Load(); clientAddr != nil {
		if _, err := s.conn.WriteToUDP(data, clientAddr); err != nil {
			log.Println("write: ", err)
		}
	}
}

func (s *Session) Incoming(p common.Packet, remoteAddr *net.UDPAddr) ([]byte, error) {
	nonce := common.MakeNonce(p.Header.Counter)
	decrypted, err := s.aesGCM.Open(nil, nonce[:], p.Payload, nil) 
	if err != nil {
		return nil, err
	}

	if remoteAddr != nil {
		s.remoteAddr.Store(remoteAddr)
	}

	return decrypted, nil
}

func (s *Session) TypedOutgoing(data []byte, packetType uint8) {
	counter := s.Counter.Add(1) - 1

	nonce := common.MakeNonce(counter)
	encrypted := s.aesGCM.Seal(nil, nonce[:], data, nil) 

	p := common.Packet{
		Header: common.Header{
			PacketType: packetType,
			PeerIndex:  s.PeerIndex,
			Counter:    counter,
		},
		Payload: encrypted,
	}

	encoded := common.EncodePacket(p)
	s.Send(encoded) // directly to UDP
}

func (s *Session) Outgoing(data []byte) {
	s.TypedOutgoing(data, common.DATA)
}

func (s *Session) RemoteAddr() string {
	return s.remoteAddr.Load().String()
}
