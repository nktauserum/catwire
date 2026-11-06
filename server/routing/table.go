package routing

import (
	"sync/atomic"
)

type Table struct {
	lookup []atomic.Pointer[Session]
}

func (t *Table) Lookup(idx uint32) *Session {
	if idx >= uint32(len(t.lookup)) {
		return nil
	}

	return t.lookup[idx].Load()
}

func (t *Table) Store(idx uint32, session *Session) {
	if idx >= uint32(len(t.lookup)) {
		return
	}

	t.lookup[idx].Store(session)
}
