package routing

import (
	"sync"
)

type PeerRouting struct {
	lookupTable map[uint32]*Session
	mu          sync.RWMutex
}

func NewPeerRouting() PeerRouting {
	return PeerRouting{
		lookupTable: make(map[uint32]*Session),
	}
}

func (pr *PeerRouting) Store(clientIP uint32, session *Session) {
	pr.mu.Lock()
	pr.lookupTable[clientIP] = session
	pr.mu.Unlock()
}

func (pr *PeerRouting) Load(destIP uint32) *Session {
	pr.mu.RLock()
	s := pr.lookupTable[destIP]
	pr.mu.RUnlock()

	return s
}
