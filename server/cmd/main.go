package main

import (
	"fmt"

	"github.com/nktauserum/catwire/common/crypto"
)

func main() {
	// p := models.CreatePool[uint64]()
	//
	// idx := p.Acquire()
	// fmt.Printf("Got index %v\n", idx)
	// p.Release(idx)
	// idx = p.Acquire()
	// fmt.Printf("Got index %v\n", idx)
	// p.Release(idx)

	var buf [32]byte
	crypto.RandomBytes(buf[:])
	fmt.Printf("%#v\n", buf)
}
