package session

import (
	"crypto/cipher"
	"crypto/ecdh"
	"net"
	"sync"
	"sync/atomic"

	"github.com/nktauserum/catwire/common"
)

type Session struct {
	secret  []byte
	aesGCM  cipher.AEAD
	Counter atomic.Uint64

	remoteAddr atomic.Pointer[net.UDPAddr]
	PublicKey  *ecdh.PublicKey

	PeerIndex uint64

	mu   sync.RWMutex
	pool sync.Pool

	incomingFunc IncomingCallback
	outgoingFunc OutgoingCallback
}

func NewSession(
	inc IncomingCallback,
	out OutgoingCallback,
	addr *net.UDPAddr,
) *Session {
	s := &Session{
		incomingFunc: inc,
		outgoingFunc: out,
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

	s.pool = sync.Pool{
		New: func() any {
			b := make([]byte, 65535+aesGCM.Overhead()) // + AES overhead
			return &b
		},
	}

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
		s.outgoingFunc(data, clientAddr)
	}
}

func (s *Session) Incoming(p common.Packet, remoteAddr *net.UDPAddr) {
	buf := s.pool.Get().(*[]byte)
	*buf = (*buf)[:len(p.Payload)-s.aesGCM.Overhead()]

	nonce := common.MakeNonce(p.Header.Counter)
	payload, err := s.aesGCM.Open(*buf, nonce[:], p.Payload, nil)
	if err != nil {
		return
	}

	if remoteAddr != nil {
		s.remoteAddr.Store(remoteAddr)
	}

	s.incomingFunc(payload)
	s.pool.Put(buf)
}

func (s *Session) TypedOutgoing(data []byte, packetType uint8) {
	buf := s.pool.Get().(*[]byte)
	*buf = (*buf)[:0]

	counter := s.Counter.Add(1) - 1
	nonce := common.MakeNonce(counter)
	encrypted := s.aesGCM.Seal(*buf, nonce[:], data, nil)

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
	s.pool.Put(buf)
}

func (s *Session) Outgoing(data []byte) {
	s.TypedOutgoing(data, common.DATA)
}

func (s *Session) RemoteAddr() string {
	return s.remoteAddr.Load().String()
}
