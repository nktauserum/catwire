package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"

	"github.com/nktauserum/catwire/common/crypto"
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
	cipher *crypto.Cipher
}

type receive_buffer struct {
	incomingAddr *net.UDPAddr
	size         int
	payload      [protocol.MaxPayloadSize]byte
}

type transmit_buffer struct {
	size    int
	payload [protocol.MaxPayloadSize]byte
}

func (s *server) receive() {
	pool := models.CreatePool[receive_buffer]()
	ch := make(chan uint32, 1024)

	for range runtime.NumCPU() {
		go func() {
			for idx := range ch {
				buf := pool.Get(idx)
				log.Printf("Receive: got %v bytes\n", buf.size)

				header := protocol.Header{
					Flags:     binary.BigEndian.Uint32(buf.payload[0:4]),
					PeerIndex: binary.BigEndian.Uint32(buf.payload[4:8]),
					Counter:   binary.BigEndian.Uint64(buf.payload[8:16]),
					Nonce:     buf.payload[16:protocol.HeaderSize],
				}

				s_ptr := s.table.Lookup(header.PeerIndex)
				if s_ptr == nil {
					pool.Release(idx)
					continue
				}

				var e bool
				if header.Flags&protocol.F_HANDSHAKE == 1 {
					log.Printf("Receive: Handshake from %v\n", buf.incomingAddr)

					if buf.size != protocol.HeaderSize+protocol.HandshakeSize {
						pool.Release(idx)
						continue
					}

					session := *s_ptr
					session.Active = true
					session.PublicAddr = *buf.incomingAddr

					session.ReceiveKey, session.TransmitKey, e = s.cipher.GenerateServerKeys()
					if e {
						pool.Release(idx)
						continue
					}

					s.table.Store(idx, &session)

					var tx_buf [protocol.HeaderSize + protocol.HandshakeSize]byte

					binary.BigEndian.PutUint32(tx_buf[0:4], protocol.F_HANDSHAKE)
					binary.BigEndian.PutUint32(tx_buf[4:8], idx)
					binary.BigEndian.PutUint64(tx_buf[8:16], 0)

					copy(tx_buf[protocol.HeaderSize:], s.cipher.PublicKey[:])

					s.conn.WriteToUDP(tx_buf[:], buf.incomingAddr)

				} else if header.Flags&protocol.F_DATA == 1 {

				}

				pool.Release(idx)
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

		if buf.size <= protocol.HeaderSize {
			pool.Release(idx)
			continue
		}

		ch <- idx
	}
}

func (s *server) transmit() {
	pool := models.CreatePool[transmit_buffer]()
	ch := make(chan uint32, 1024)

	for range runtime.NumCPU() {
		go func() {
			for idx := range ch {
				buf := pool.Get(idx)
				log.Printf("Transmit: got %v bytes\n", buf.size)
				pool.Release(idx)
			}
		}()
	}

	var err error
	for {
		idx := pool.Acquire()
		buf := pool.Get(idx)

		buf.size, err = s.tunnel.Read(buf.payload[:])
		if err != nil {
			pool.Release(idx)
			continue
		}

		ch <- idx
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
