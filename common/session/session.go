package session

import (
	"net"

	"github.com/nktauserum/catwire/common/crypto"
	"github.com/nktauserum/catwire/common/protocol"
)

type Session struct {
	Active                             bool
	PublicKey, ReceiveKey, TransmitKey [32]byte
	PublicAddr                         *net.UDPAddr
	LocalAddr                          net.IP
	PeerIndex                          uint32
}

func (s *Session) Decrypt(dst, src []byte) (uint64, bool) {
	return crypto.Decrypt(
		dst,
		src[protocol.HeaderSize:],
		src[:protocol.HeaderSize],
		src[protocol.HeaderSize-32:protocol.HeaderSize],
		s.ReceiveKey,
	)
}

func (s *Session) Encrypt(dst, src []byte) (uint64, bool) {
	return crypto.Encrypt(
		dst[protocol.HeaderSize:],
		src,
		dst[:protocol.HeaderSize],
		dst[protocol.HeaderSize-32:protocol.HeaderSize],
		s.TransmitKey,
	)
}
