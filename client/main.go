package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"

	"github.com/nktauserum/catwire/common/crypto"
	"github.com/nktauserum/catwire/common/interface/tun"
	"github.com/nktauserum/catwire/common/models"
	"github.com/nktauserum/catwire/common/protocol"
	"golang.org/x/sys/unix"
)

type client struct {
	conn   *net.UDPConn
	tunnel *tun.TUN
	cipher *crypto.Cipher

	session                         atomic.Pointer[session.Session] // should be initialized before the start of workers
	receiveCounter, transmitCounter atomic.Uint64
}

type buffer struct {
	incomingAddr *net.UDPAddr
	size         int
	payload      [protocol.MaxPayloadSize]byte
}

func (c *client) receive() {
	pool := models.CreatePool[buffer]()
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

				var e bool
				if header.Flags&protocol.F_HANDSHAKE == 1 {
					log.Println("Receive: Handshake response")

					if buf.size != protocol.HeaderSize+protocol.HandshakeSize {
						pool.Release(idx)
						continue
					}

					session := *c.session.Load()
					session.ReceiveKey, session.TransmitKey, e = c.cipher.GenerateClientKeys(buf.payload[protocol.HeaderSize:])
					if e {
						pool.Release(idx)
						continue
					}

					session.Active = true
					copy(session.PublicKey, buf.payload[protocol.HeaderSize:])

					c.session.Store(session)

				} else if header.Flags&protocol.F_DATA == 1 {
					session := c.session.Load()
					if !session.Active {
						pool.Release(idx)
						continue
					}

					var outBuf buffer
					decrypted, e := session.Decrypt(outBuf.payload[:], buf.payload[:])
					if e {
						pool.Release(idx)
						continue
					}

					c.tunnel.Write(decrypted) // TODO: add internal routing
					c.receiveCounter.Add(1)
				}

				pool.Release(idx)
			}
		}()
	}

	log.Printf("Receive loop started: %v workers", runtime.NumCPU())

	var err error
	for {
		idx := pool.Acquire()
		buf := pool.Get(idx)

		buf.size, buf.incomingAddr, err = c.conn.ReadFromUDP(buf.payload[:])
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

func (c *client) transmit() {
	pool := models.CreatePool[buffer]()
	ch := make(chan uint32, 1024)

	for range runtime.NumCPU() {
		go func() {
			for idx := range ch {
				buf := pool.Get(idx)
				log.Printf("Transmit: got %v bytes\n", buf.size)

				session := *c.session.Load()
				if !session.Active {
					pool.Release(idx)
					continue
				}

				var nonce [32]byte
				var outBuf buffer
				crypto.RandomBytes(nonce[:])

				encrypted, e := session.Encrypt(outBuf.payload[:], buf.payload[:])
				if e {
					pool.Release(idx)
					continue
				}

				pool.Release(idx)

				c.conn.WriteToUDP(encrypted, session.PublicAddr)
			}
		}()
	}

	log.Printf("Transmit loop started: %v workers", runtime.NumCPU())

	var err error
	for {
		idx := pool.Acquire()
		buf := pool.Get(idx)

		buf.size, err = c.tunnel.Read(buf.payload[:])
		if err != nil {
			pool.Release(idx)
			continue
		}

		ch <- idx
	}
}

func main() {
	e := crypto.Init()
	if e {
		log.Fatalln("error initializing crypto")
	}

	c, err := config.ParseConfig("server/config/config.example.ini")
	if err != nil {
		log.Fatalln(err)
	}

	cipher, e := crypto.CreateCipher(c.Seed)
	if e {
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

	s := client{conn, tunnel, cipher}

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
