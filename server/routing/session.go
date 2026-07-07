package routing

import (
	"crypto/ecdh"
	"log"
	"net"
	"sync/atomic"

	"github.com/nktauserum/catwire/common"
)

const ipAddr = "10.0.5.1"                                 // as a server
const subnetMask = (0xFFFFFFFF << (32 - 24)) & 0xFFFFFFFF // 24 as CIDR notation (0xFFFFFF00)

var subnetAddr = getIPSubnet(common.IPAsInteger(ipAddr), subnetMask)

type Session struct {
	PublicKey *ecdh.PublicKey
	secret    []byte

	outgoing chan []byte

	crypto *common.Crypto

	remoteAddr atomic.Pointer[net.UDPAddr]
	conn       *net.UDPConn
	PeerIndex  uint64
	Counter    atomic.Uint64

	IPLookupTable *PeerRouting
}

func NewSession(
	outgoing chan []byte,
	conn *net.UDPConn,
	addr *net.UDPAddr,
	table *PeerRouting,
) *Session {
	s := &Session{
		outgoing:      outgoing,
		conn:          conn,
		IPLookupTable: table,
	}

	s.remoteAddr.Store(addr)
	return s
}

func (s *Session) InitSession(
	idx uint64,
	key *ecdh.PublicKey,
	crypto *common.Crypto,
) {
	s.PeerIndex = idx
	s.crypto = crypto
	s.PublicKey = key
}

func (s *Session) Send(data []byte) {
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
	if IPInLocalSubnet(destIP) && destIP != common.IPAsInteger(ipAddr) { // only if destIP owned by our virtual network and it isn't server's address (because it doesn't exist in IPLookupTable)
		session := s.IPLookupTable.Load(destIP)

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
			PeerIndex:  s.PeerIndex,
			Counter:    counter,
		},
		Payload: encrypted,
	}

	encoded := common.EncodePacket(p)
	s.Send(encoded) // directly to UDP
}
