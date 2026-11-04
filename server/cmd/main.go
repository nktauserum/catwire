package main

import "github.com/nktauserum/catwire/common/models/pool"

func main() {
	p := pool.CreatePool[uint64]()
	idx := p.Acquire()
	p.Release(idx)
}
