package main

import (
	"log"

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

	if !crypto.Init() {
		log.Fatalln("Error initializing the cryptography")
	}
}
