package routing

import (
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/nktauserum/catwire/common/session"
)

type IndexTable struct {
	lookupTable []*session.Session
	mu          sync.RWMutex
}

func NewIndexTable(cap int) IndexTable {
	return IndexTable{
		lookupTable: make([]*session.Session, 0, cap),
	}
}

func (t *IndexTable) Load(peerIndex uint64) (*session.Session, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	p := peerIndex - 1

	if p >= uint64(len(t.lookupTable)) {
		return nil, fmt.Errorf("no such peerIndex")
	}

	s := t.lookupTable[p]
	if s == nil {
		return nil, fmt.Errorf("equals nil")
	}

	return s, nil
}

func (t *IndexTable) Store(key string, session *session.Session) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	// compare already existing and incoming  using the encoded private key
	for i := range t.lookupTable { // O(n) but acceptable for rare handshakes
		k := base64.StdEncoding.EncodeToString(
			t.lookupTable[i].PublicKey.Bytes(),
		)

		if k == key {
			idx := uint64(i + 1)
			t.lookupTable[i] = session

			return idx
		}
	}

	// if it doesn't exist, create a new entry
	idx := uint64(len(t.lookupTable)) + 1
	t.lookupTable = append(t.lookupTable, session)

	return idx
}
