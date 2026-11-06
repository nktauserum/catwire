package routing

import (
	"net"

	"github.com/nktauserum/catwire/common/crypto"
)

type Session struct {
	Active                             bool
	PublicKey, ReceiveKey, TransmitKey [32]byte
	PublicAddr                         net.UDPAddr
	LocalAddr                          net.IP
	ReceiveCounter, TransmitCounter    uint64 // TODO: move to the routing table
}

func (s *Session) Decrypt(dst, src []byte) ([]byte, bool) {
	return crypto.Decrypt(dst, src, s.ReceiveKey)
}
