package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"

	"github.com/nktauserum/catwire/common/interface/tun"
	"github.com/nktauserum/catwire/common/models"
	"github.com/nktauserum/catwire/common/protocol"
	"github.com/nktauserum/catwire/server/config"
	"github.com/nktauserum/catwire/server/routing"
	"golang.org/x/sys/unix"
)

type server struct {
	conn   *net.UDPConn
	tunnel *tun.TUN
	table  *routing.Table
}

type buffer struct {
	incomingAddr *net.UDPAddr
	size         int
	payload      [protocol.MaxPayloadSize]byte
}

func (s *server) receive() {
	pool := models.CreatePool[buffer]()
	ch := make(chan uint32, 1024)

	for range runtime.NumCPU() {
		go func() {
			for idx := range ch {
				buf := pool.Get(idx)
				log.Printf("Receive: got %v bytes\n", buf.size)
			}
		}()
	}

	var err error
	for {
		idx := pool.Acquire()
		buf := pool.Get(idx)

		buf.size, buf.incomingAddr, err = s.conn.ReadFromUDP(buf.payload[:])
		if err != nil {
			pool.Release(idx)
			continue
		}

		ch <- idx
	}
}

func (s *server) transmit() {
	for {
	}
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

	s := server{conn, tunnel, table}

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
