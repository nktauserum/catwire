package models

import (
	"net"

	"github.com/nktauserum/catwire/common/protocol"
)

type Buffer struct {
	IncomingAddr *net.UDPAddr
	Size         int
	Payload      [protocol.MaxPayloadSize]byte
}
