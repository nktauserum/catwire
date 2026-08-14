package routing

import (
	"maps"
	"sync"

	"github.com/nktauserum/catwire/common/session"
)

type AddressTable struct {
	lookupTable map[uint32]*session.Session
	mu          sync.RWMutex
}

func NewAddressTable() AddressTable {
	return AddressTable{
		lookupTable: make(map[uint32]*session.Session),
	}
}

func (t *AddressTable) Store(clientIP uint32, session *session.Session) {
	t.mu.Lock()
	t.lookupTable[clientIP] = session
	t.mu.Unlock()
}

func (t *AddressTable) Load(destIP uint32) *session.Session {
	t.mu.RLock()
	s := t.lookupTable[destIP]
	t.mu.RUnlock()

	return s
}

func (t *AddressTable) Copy() map[uint32]*session.Session {
	t.mu.RLock()
	ret := make(map[uint32]*session.Session, len(t.lookupTable))
	maps.Copy(ret, t.lookupTable)
	t.mu.RUnlock()

	return ret
}
