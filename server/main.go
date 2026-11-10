package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
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

const ipAddr = "10.0.5.1"

type server struct {
	conn   *net.UDPConn
	tunnel *tun.TUN
	table  *routing.Table
	cipher *crypto.Cipher
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

				header := protocol.Header{
					Flags:     binary.BigEndian.Uint32(buf.payload[0:4]),
					PeerIndex: binary.BigEndian.Uint32(buf.payload[4:8]),
					Counter:   binary.BigEndian.Uint64(buf.payload[8:16]),
					Nonce:     buf.payload[16:protocol.HeaderSize],
				}

				entry := s.table.Lookup(header.PeerIndex)
				if entry == nil {
					pool.Release(idx)
					continue
				}

				if header.Flags&protocol.F_HANDSHAKE != 0 {
					if buf.size != protocol.HeaderSize+protocol.HandshakeSize {
						pool.Release(idx)
						continue
					}

					idx, e := s.table.Find(buf.payload[protocol.HeaderSize:buf.size])
					if e {
						log.Printf("Couldn't find an available session for %v\n", buf.incomingAddr)
						pool.Release(idx)
						continue
					}

					entry = s.table.Lookup(idx)
					if entry == nil {
						pool.Release(idx)
						continue
					}

					s_ptr := entry.Session.Load()
					if s_ptr == nil {
						pool.Release(idx)
						continue
					}

					session := *s_ptr
					session.Active = true
					session.PublicAddr = buf.incomingAddr

					session.ReceiveKey, session.TransmitKey, e = s.cipher.GenerateServerKeys(buf.payload[protocol.HeaderSize:])
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

					log.Printf("Receive: Handshake from %v\n", buf.incomingAddr)

				} else if header.Flags&protocol.F_DATA != 0 {
					s_ptr := entry.Session.Load()
					if s_ptr == nil {
						pool.Release(idx)
						continue
					}

					session := *s_ptr
					if !session.Active {
						pool.Release(idx)
						continue
					}

					var outBuf buffer
					decrypted, e := session.Decrypt(outBuf.payload[:], buf.payload[:buf.size])
					if e {
						pool.Release(idx)
						continue
					}

					s.tunnel.Write(outBuf.payload[:decrypted]) // TODO: add internal routing
					entry.TransmitCounter.Add(1)

					if session.PublicAddr != buf.incomingAddr {
						if old := *entry.Session.Load(); old.PublicAddr != buf.incomingAddr { // TODO: add replay check
							old.PublicAddr = buf.incomingAddr
							s.table.Store(header.PeerIndex, &old)
						}
					}
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
	pool := models.CreatePool[buffer]()
	ch := make(chan uint32, 1024)

	for range runtime.NumCPU() {
		go func() {
			for idx := range ch {
				buf := pool.Get(idx)
				destIP := routing.ExtractDestinationIP(buf.payload[:])
				peerIndex, ok := routing.ExtractIndex(destIP)
				if !ok {
					pool.Release(idx)
					continue
				}

				entry := s.table.Lookup(peerIndex)
				if entry == nil {
					pool.Release(idx)
					continue
				}

				s_ptr := entry.Session.Load()
				if s_ptr == nil {
					pool.Release(idx)
					continue
				}

				session := *s_ptr
				if !session.Active {
					pool.Release(idx)
					continue
				}

				var outBuf buffer
				binary.BigEndian.PutUint32(outBuf.payload[0:4], protocol.F_DATA)
				binary.BigEndian.PutUint32(outBuf.payload[4:8], session.PeerIndex)
				binary.BigEndian.PutUint64(outBuf.payload[8:16], entry.TransmitCounter.Add(1)-1)
				crypto.RandomBytes(outBuf.payload[16:protocol.HeaderSize])

				encrypted, e := session.Encrypt(outBuf.payload[:], buf.payload[:buf.size])
				if e {
					log.Println("Error encrypt the packet")
					pool.Release(idx)
					continue
				}

				pool.Release(idx)

				s.conn.WriteToUDP(outBuf.payload[:encrypted+uint64(protocol.HeaderSize)], session.PublicAddr)

			}
		}()
	}

	log.Printf("Transmit loop started: %v workers", runtime.NumCPU())

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
	if len(os.Args) <= 1 {
		log.Fatalln("pls provide a config")
	}

	e := crypto.Init()
	if e {
		log.Fatalln("error initializing crypto")
	}

	c, err := config.ParseConfig(os.Args[1])
	if err != nil {
		log.Fatalln(err)
	}

	table := routing.CreateTable(c.Clients)
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

	cmds := [][]string{
		{"ip", "link", "set", "cw0", "up"},
		{"ip", "addr", "add", ipAddr + "/24", "dev", "cw0"},
		{"ip", "link", "set", "dev", "cw0", "mtu", "1420"},
		{"iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE"},
		{"iptables", "-A", "FORWARD", "-i", "cw0", "-j", "ACCEPT"},
		{"iptables", "-A", "FORWARD", "-o", "cw0", "-j", "ACCEPT"},
		{"sysctl", "-w", "net.ipv4.ip_forward=1"},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run %v: %v, output: %s", cmd, err, string(out))
		}
	}

	s := server{conn, tunnel, table, cipher}

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
