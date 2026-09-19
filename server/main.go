package main

import (
	"log"
	"net"

	"github.com/nktauserum/catwire/server/io"
)

func main() {
	ring, err := io.NewRingDefault()
	if err != nil {
		log.Fatalf("Error creating new ring: %v\n", err.Error())
	}

	pool, err := io.CreateInternalPool(ring.Fd())
	if err != nil {
		log.Fatalf("Error creating new pool: %v\n", err.Error())
	}

	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:45230")
	if err != nil {
		log.Fatalf("Error resolving udp addr: %v\n", err.Error())
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		log.Fatalf("Error listening udp: %v\n", err.Error())
	}

	f, err := conn.File()
	if err != nil {
		log.Fatalf("Error copying file: %v\n", err.Error())
	}

	_, errno := ring.SubmitMultishot(pool, int32(f.Fd()))
	if errno != 0 {
		log.Fatalf("Error multishot: %v\n", errno.Error())
	}

	for {
		e := ring.WaitForCQ()
		if e != 0 {
			continue
		}

		for {
			cqe, ok := ring.ReadFromCQ()
			if !ok {
				break
			}

			log.Printf("%#v\n", cqe)
		}
	}
}
