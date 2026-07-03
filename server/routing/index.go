package routing

import (
	"encoding/base64"
	"fmt"
	"sync"
)

type PeerIndices struct {
	lookupTable []*Session
	mu          sync.Mutex
}

func NewPeerIndices(cap int) PeerIndices {
	return PeerIndices{
		lookupTable: make([]*Session, 0, cap),
	}
}

func (pi *PeerIndices) Load(peerIndex uint64) (*Session, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	p := peerIndex - 1

	if p >= uint64(len(pi.lookupTable)) {
		return nil, fmt.Errorf("no such peerIndex")
	}

	s := pi.lookupTable[p]

	if s == nil {
		return nil, fmt.Errorf("equals nil")
	}

	return s, nil
}

func (pi *PeerIndices) Store(key string, session *Session) uint64 {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	// compare already existing and incoming  using the encoded private key
	for i := range pi.lookupTable { // O(n) but acceptable for rare handshakes
		k := base64.StdEncoding.EncodeToString(
			pi.lookupTable[i].PublicKey.Bytes(),
		)

		if k == key {
			idx := uint64(i + 1)
			pi.lookupTable[i] = session

			return idx
		}
	}

	// if it doesn't exist, create a new entry
	idx := uint64(len(pi.lookupTable)) + 1
	pi.lookupTable = append(pi.lookupTable, session)

	return idx
}
