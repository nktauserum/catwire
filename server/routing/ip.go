package routing

import (
	"sync"

	"github.com/nktauserum/catwire/server/session"
)

type PeerRouting struct {
	lookupTable map[uint32]*session.Session
	mu          sync.RWMutex
}

func NewPeerRouting() PeerRouting {
	return PeerRouting{
		lookupTable: make(map[uint32]*session.Session),
	}
}

func (pr *PeerRouting) Store(clientIP uint32, session *session.Session) {
	pr.mu.Lock()
	pr.lookupTable[clientIP] = session
	pr.mu.Unlock()
}

func (pr *PeerRouting) Load(destIP uint32) *session.Session {
	pr.mu.RLock()
	s := pr.lookupTable[destIP]
	pr.mu.RUnlock()

	return s
}
