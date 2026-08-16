package routing

import (
	"crypto/ecdh"
	"fmt"
	"net"
	"sync"

	"github.com/nktauserum/catwire/common/session"
)

var (
	ErrPeerNotFound error = fmt.Errorf("error peer not found")
)

type Peer struct {
	RemoteAddress *net.UDPAddr
	PublicKey     *ecdh.PublicKey
	Session       *session.Session

	mu sync.Mutex
}

func NewPeer(remote *net.UDPAddr, session *session.Session) Peer {
	return Peer{RemoteAddress: remote, Session: session}
}

func (p *Peer) Connected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.Session != nil
}

type PeerTable struct {
	mu        sync.RWMutex
	peerTable map[uint32]*Peer
}

func NewPeerTable() PeerTable {
	return PeerTable{
		peerTable: make(map[uint32]*Peer),
	}
}

func (t *PeerTable) Add(privateAddr uint32, publicAddr *net.UDPAddr, publicKey *ecdh.PublicKey) {
	peer := &Peer{RemoteAddress: publicAddr, PublicKey: publicKey, Session: nil}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.peerTable[privateAddr] = peer
}

func (t *PeerTable) Get(addr uint32) (*Peer, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	peer, exists := t.peerTable[addr]
	if !exists {
		return nil, ErrPeerNotFound
	}

	return peer, nil
}

func (t *PeerTable) Find(publicKey *ecdh.PublicKey) (uint32, *Peer, error) {
	privateAddr := uint32(0)
	foundPeer := new(Peer)

	t.mu.RLock()
	// yeah, we definitely need O(n) search on a hash table (this is not a bottleneck)
	for addr, peer := range t.peerTable { // maybe consider creating reverse index
		if peer.PublicKey == publicKey {
			privateAddr = addr
			foundPeer = peer
			break
		}
	}
	t.mu.RUnlock()

	if privateAddr == 0 {
		return 0, foundPeer, ErrPeerNotFound
	}

	return privateAddr, foundPeer, nil
}
