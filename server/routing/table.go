package routing

import (
	"bytes"
	"log"
	"sync/atomic"

	"github.com/nktauserum/catwire/common/session"
	"github.com/nktauserum/catwire/server/config"
)

const availableClients = 256

type Entry struct {
	ReceiveCounter, TransmitCounter atomic.Uint64
	Session                         atomic.Pointer[session.Session]
}

type Table struct {
	lookup [availableClients]Entry
}

func CreateTable(clients []config.Client) *Table {
	var table Table

	for _, c := range clients {
		addr, err := IPAsInteger(c.LocalAddr)
		if err {
			continue
		}

		idx, err := ExtractIndex(addr)
		if err {
			log.Printf("Error adding the entry: idx %v, addr %v\n", idx, c.LocalAddr)
			continue
		}

		session := &session.Session{
			PublicKey: c.PublicKey,
			LocalAddr: c.LocalAddr,
			Active:    false,
		}

		log.Printf("Entry: idx %v, addr %v\n", idx, c.LocalAddr)

		table.Store(idx, session)
	}

	return &table
}

func (t *Table) Find(key []byte) (uint32, bool) {
	for idx, _ := range t.lookup {
		s_ptr := t.lookup[idx].Session.Load()
		if s_ptr == nil {
			continue
		}

		if bytes.Equal(s_ptr.PublicKey[:], key) {
			return uint32(idx), false
		}
	}

	return 0, true
}

func (t *Table) Lookup(idx uint32) *Entry {
	if idx >= availableClients {
		return nil
	}

	return &t.lookup[idx]
}

func (t *Table) Store(idx uint32, session *session.Session) {
	if idx >= availableClients {
		return
	}

	t.lookup[idx].Session.Store(session)
}
