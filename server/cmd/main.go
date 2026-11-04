package main

import (
	"fmt"

	"github.com/nktauserum/catwire/common/models"
)

func main() {
	p := models.CreatePool[uint64]()

	for range 65 {
		idx := p.Acquire()
		fmt.Printf("Got index %v\n", idx)
		p.Release(idx)
	}
	// p.Release(idx)
}
