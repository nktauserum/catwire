package session

import (
	"crypto/ecdh"
	"log"
	"net"
	"sync/atomic"

	"github.com/nktauserum/catwire/common"
)

type Session struct {
	secret  []byte
	crypto  *common.Crypto
	Counter atomic.Uint64

	remoteAddr atomic.Pointer[net.UDPAddr]
	conn       *net.UDPConn
	PublicKey  *ecdh.PublicKey

	PeerIndex uint64
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
	crypto *common.Crypto,
	publicKey *ecdh.PublicKey,
) {
	s.PeerIndex = idx
	s.crypto = crypto
	s.PublicKey = publicKey
}

func (s *Session) Send(data []byte) {
	if clientAddr := s.remoteAddr.Load(); clientAddr != nil {
		if _, err := s.conn.WriteToUDP(data, clientAddr); err != nil {
			log.Println("write: ", err)
		}
	}
}

func (s *Session) Incoming(p common.Packet, remoteAddr *net.UDPAddr) ([]byte, error) {
	decrypted, err := s.crypto.Decrypt(p.Payload, p.Header.Counter)
	if err != nil {
		return nil, err
	}

	if remoteAddr != nil {
		s.remoteAddr.Store(remoteAddr)
	}

	return decrypted, nil
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
			PeerIndex:  s.PeerIndex,
			Counter:    counter,
		},
		Payload: encrypted,
	}

	encoded := common.EncodePacket(p)
	s.Send(encoded) // directly to UDP
}

func (s *Session) RemoteAddr() string {
	return s.remoteAddr.Load().String()
}
