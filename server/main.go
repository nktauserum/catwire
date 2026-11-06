package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/nktauserum/catwire/common/interface/tun"
	"github.com/nktauserum/catwire/server/config"
	"github.com/nktauserum/catwire/server/routing"
	"golang.org/x/sys/unix"
)

type server struct {
	conn *net.UDPConn
	tunnel *tun.TUN
	table *routing.Table
}

func (s *server) receive() {
	for {}
}

func (s *server) transmit() {
	for {}
}

func main() {
	c, err := config.ParseConfig("server/config/config.example.ini")
	if err != nil {
		log.Fatalln(err)
	}

	table := routing.CreateTable(c.Clients)

	conn, err := net.ListenUDP("udp", &c.Address)
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	tunnel, err := tun.CreateTUN("cw0")
	if err != nil {
		log.Fatalf("error creating TUN: %v\n", err)
	}

	s := server {conn, tunnel, table}

	go s.receive()
	go s.transmit()

	term := make(chan os.Signal, 1)

	signal.Notify(term, unix.SIGTERM)
	signal.Notify(term, os.Interrupt)

	select {
	case <-term:
	}

	fmt.Println("\nShutdown")
}
