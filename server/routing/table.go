package routing

import (
	"sync/atomic"

	"github.com/nktauserum/catwire/server/config"
)

const availableClients = 256

type Table struct {
	lookup [availableClients]atomic.Pointer[Session]
}

func CreateTable(clients []config.Client) *Table {
	var table Table

	for _, c := range clients {
		idx, err := ExtractIndex(c.LocalAddr)
		if err {
			continue
		}

		session := &Session{
			PublicKey: c.PublicKey,
			LocalAddr: c.LocalAddr,
			Active:    false,
		}

		table.Store(idx, session)
	}

	return &table
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
