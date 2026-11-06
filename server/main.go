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

func receive(conn *net.UDPConn, table *routing.Table) {
	for {}
}

func transmit(tunnel *tun.TUN, table *routing.Table) {
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

	go receive(conn, table)
	go transmit(tunnel, table)

	term := make(chan os.Signal, 1)

	signal.Notify(term, unix.SIGTERM)
	signal.Notify(term, os.Interrupt)

	select {
	case <-term:
	}

	fmt.Println("\nShutdown")
}
