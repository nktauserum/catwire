package routing

import (
	"sync/atomic"

	"github.com/nktauserum/catwire/server/config"
)

const availableClients = 256

type Entry struct {
	ReceiveCounter, TransmitCounter atomic.Uint64
	Session                         atomic.Pointer[Session]
}

type Table struct {
	lookup [availableClients]Entry
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

func (t *Table) Lookup(idx uint32) *Entry {
	if idx >= availableClients {
		return nil
	}

	return &t.lookup[idx]
}

func (t *Table) Store(idx uint32, session *Session) {
	if idx >= availableClients {
		return
	}

	t.lookup[idx].Session.Store(session)
}
