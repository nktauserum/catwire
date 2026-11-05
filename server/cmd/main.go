package main

import (
	"log"
	"net"

	"github.com/nktauserum/catwire/common/interface/tun"
)

func main() {
	addr, err := net.ResolveUDPAddr("udp", ":55352")
	if err != nil {
		log.Fatalf("error resolving udp addr: %v\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	_, err = tun.CreateTUN("cw0")
	if err != nil {
		log.Fatalf("error creating TUN: %v\n", err)
	}
}
