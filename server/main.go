package main

import (
	"log"
	"net"

	"github.com/nktauserum/catwire/common/interface/tun"
	"github.com/nktauserum/catwire/server/config"
)

func main() {
	c, err := config.ParseConfig("server/config/config.example.ini")
	if err != nil {
		log.Fatalln(err)
	}

	conn, err := net.ListenUDP("udp", &c.Address)
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	tunnel, err := tun.CreateTUN("cw0")
	if err != nil {
		log.Fatalf("error creating TUN: %v\n", err)
	}
}
