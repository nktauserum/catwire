package routing

import (
	"net"
	"sync/atomic"
)

type Session struct {
	Active                             bool
	PublicKey, ReceiveKey, TransmitKey [32]byte
	PublicAddr, LocalAddr              net.IP
	ReceiveCounter, TransmitCounter    atomic.Uint64
}
