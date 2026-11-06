package routing

import (
	"net"
)

type Session struct {
	Active                             bool
	PublicKey, ReceiveKey, TransmitKey [32]byte
	PublicAddr                         net.UDPAddr
	LocalAddr                          net.IP
	ReceiveCounter, TransmitCounter    uint64
}
