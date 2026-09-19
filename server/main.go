package main

import (
	"log"
	"net"
	"syscall"

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

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		log.Fatalf("Error creating socket: %v\n", err.Error())
	}

	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		log.Fatalf("Cannot set SO_REUSEADDR on socket, %s", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", ":43250")
	if err != nil && udpAddr.IP != nil {
		log.Fatalf("Cannot resolve addr, %s", err)
	}

	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Port: udpAddr.Port}); err != nil {
		log.Fatalf("Cannot bind socket, %s", err)
	}

	_, errno := ring.SubmitMultishot(pool, int32(fd))
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
