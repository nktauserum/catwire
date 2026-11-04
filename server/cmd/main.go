package main

import (
	"fmt"

	"github.com/nktauserum/catwire/common/models"
)

func main() {
	p := models.CreatePool[uint64]()
	idx := p.Acquire()
	fmt.Printf("Got index %v\n", idx)
	idx = p.Acquire()
	fmt.Printf("Got index %v\n", idx)
	idx = p.Acquire()
	fmt.Printf("Got index %v\n", idx)
	// p.Release(idx)
}
