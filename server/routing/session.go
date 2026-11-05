package routing

import "sync/atomic"

type Session struct {
	PublicKey, ReceiveKey, TransmitKey [32]byte
	PublicAddr, LocalAddr              uint32
	ReceiveCounter, TransmitCounter    atomic.Uint64
}
