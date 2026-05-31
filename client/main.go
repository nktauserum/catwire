package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"log"
	"net"
	"os/exec"
	"sync/atomic"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/common"
)

const (
	ipAddr    = "10.0.5.2"
	peer      = "10.0.5.1"
	serverUDP = "192.168.1.2:55635"
)

var nextSequenceNumber atomic.Uint64

type State uint8

const (
	StateHandshakeInit State = iota
	StateWaitHandshakeResp
	StateHandshakeFinish
	StateWaiting
	StateTransmit
)

type Client struct {
	s State

	serverPublicKey *ecdh.PublicKey
	secret          []byte

	incoming 		chan common.Packet
	outgoing        chan []byte

	curve            ecdh.Curve
	clientPrivateKey *ecdh.PrivateKey
	clientPublicKey  *ecdh.PublicKey
}

func (s *Client) Start() {
	for {
		switch s.s {
		case StateHandshakeInit:
			p := common.Packet{
				Header: common.Header{
					PacketType: common.HANDSHAKE_INIT,
					PeerIndex:  0,
					Counter:    nextSequenceNumber.Load(),
				},
				Payload: s.clientPublicKey.Bytes(),
			}

			encHandshake := common.EncodePacket(p)

			s.outgoing <- encHandshake
			s.s = StateWaitHandshakeResp

		case StateWaitHandshakeResp:
			p := <-s.incoming

			if p.Header.PacketType != common.HANDSHAKE_INIT {
				continue
			}

			var err error

			s.serverPublicKey, err = s.curve.NewPublicKey(p.Payload)
			if err != nil {
				log.Printf("error creating new public key: %v\n", err)
				continue
			}

			s.secret, err = s.clientPrivateKey.ECDH(s.serverPublicKey)
			if err != nil {
				log.Printf("error computing the secret: %v\n", err)
				continue
			}

			log.Printf("secret: %v\n", s.secret)
			
			s.s = StateTransmit

		case StateTransmit: 
			break	
		}

	}

	select {}
}

func main() {
	c := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: "cw0",
		},
	}

	tun, err := water.New(c)
	if err != nil {
		log.Fatalln("error create tun: ", err)
	}

	cmds := [][]string{
		{"ip", "link", "set", tun.Name(), "up"},
		{"ip", "addr", "add", ipAddr + "/32", "peer", peer, "dev", tun.Name()},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run %v: %v, output: %s", cmd, err, string(out))
		}
	}

	log.Println("Successfully created TUN interface")

	curve := ecdh.X25519()
	clientPrivateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("error generating client private key: %v\n", err)
	}
	clientPublicKey := clientPrivateKey.PublicKey()

	conn, err := net.Dial("udp", serverUDP)
	if err != nil {
		log.Fatalln("error dialing to the server: ", err)
	}
	defer conn.Close()

	log.Printf("Dialing the connection to the server on %s\n", serverUDP)

	incoming := make(chan common.Packet)
	outgoing := make(chan []byte)

	client := Client {
		s: StateHandshakeInit,

		serverPublicKey: nil,
		secret: nil,

		outgoing: outgoing,
		incoming: incoming,

		curve: curve,
		clientPrivateKey: clientPrivateKey,
		clientPublicKey: clientPublicKey,
	}

	// start send loop
	go sendUDP(outgoing, conn)
	go listenUDP(conn, tun, incoming)
	go listenTUN(tun, outgoing)

	client.Start()
}

func listenTUN(tun *water.Interface, outgoing chan []byte) {
	buf := make([]byte, 65535)
	p := common.Packet{
		Header: common.Header{
			PacketType: common.DATA,
			PeerIndex:  0,
			Counter:    0,
		},
		Payload: nil,
	}

	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Printf("read: %v\n", err)
			continue
		}

		p.Header.PacketType = common.DATA
		p.Header.PeerIndex = 0
		p.Header.Counter = nextSequenceNumber.Load()
		p.Payload = buf[:n]

		encodedPacket := common.EncodePacket(p)
		nextSequenceNumber.Add(1)

		outgoing <- encodedPacket
	}
}

func sendUDP(outgoing chan []byte, conn net.Conn) {
	for packet := range outgoing {
		if _, err := conn.Write(packet); err != nil {
			log.Printf("write: %v", err)
			continue
		}
	}
}

func listenUDP(conn net.Conn, tun *water.Interface, incoming chan common.Packet) {
	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("error reading from %s: %v\n", conn.RemoteAddr(), err)
			continue
		}

		p := common.DecodePacket(buf[:n])

		if p.Header.PacketType == common.DATA {
			if _, err = tun.Write(p.Payload); err != nil {
				log.Printf("error writing to TUN: %v\n", err)
			}
			continue
		}

		incoming <- p
	}
}
