package main

import (
	"log"
	"os/exec"
	"sync/atomic"
	"net"
	"crypto/ecdh"
	"crypto/rand"

	"github.com/songgao/water"

	"github.com/nktauserum/catwire/common"
)

const ipAddr = "10.0.5.1"
const peer = "10.0.5.2"
var remoteAddr atomic.Pointer[net.UDPAddr] 
var nextSequenceNumber atomic.Uint64

func main() {
	c := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams {
			Name: "cw1",
		},
	}

	tun, err := water.New(c)
	if err != nil {
		log.Fatalln("error create tun: ", err)
	}

	cmds := [][]string{
		{"ip", "link", "set", tun.Name(), "up"},
		{"ip", "addr", "add", ipAddr+"/32", "peer", peer, "dev", tun.Name()},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run %v: %v, output: %s", cmd, err, string(out))
		}
	}

	// init crypto
	curve := ecdh.X25519()
	serverPrivate, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("error generating server private key: %v\n", err)
	}
	// serverPublic := serverPrivate.PublicKey().Bytes()

	addr, err := net.ResolveUDPAddr("udp", ":55635")
	if err != nil {
		log.Fatalf("error resolving udp addr: %v\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)	
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	incoming := make(chan common.Packet)
	outgoing := make(chan []byte)

	go sendUDP(conn, outgoing)
	go listenUDP(tun, conn, incoming)
	go listenTUN(tun, outgoing)

	for p := range incoming {
		if p.Header.PacketType == common.HANDSHAKE_INIT {
			clientPublicKey, err := curve.NewPublicKey(p.Payload)
			if err != nil {
				log.Printf("error creating new public key: %v\n", err)
				return
			}

			secret, err := serverPrivate.ECDH(clientPublicKey)
			if err != nil {
				log.Printf("error computing the secret: %v\n", err)
				return
			}

			log.Printf("secret: %s\n", secret)
		}
	}
}


func listenUDP(tun *water.Interface, conn *net.UDPConn, incoming chan common.Packet) {
	buf := make([]byte, 65535)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("read: ", err)
			continue
		}

		remoteAddr.Store(clientAddr)

		p := common.DecodePacket(buf[:n])

		if p.Header.PacketType == common.DATA {
			if _, err = tun.Write(p.Payload); err != nil {
				log.Println("read: ", err)
			}
			continue
		}

		incoming <- p
	}
}

func listenTUN(tun *water.Interface, outgoing chan []byte) {
	buf := make([]byte, 65535);

	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Println("write: ", err)
			continue
		} 

		p := common.Packet {
			Header: common.Header {
				PacketType: common.DATA,	
				PeerIndex: 0,
				Counter: nextSequenceNumber.Load(),
			},
			Payload: buf[:n],
		}

		encodedPacket := common.EncodePacket(p)
		nextSequenceNumber.Add(1)

		outgoing <- encodedPacket 
	}
}

func sendUDP(conn *net.UDPConn, outgoing chan []byte) {
	for packet := range outgoing {
		if clientAddr := remoteAddr.Load(); clientAddr != nil {
			if _, err := conn.WriteToUDP(packet, clientAddr); err != nil {
				log.Println("write: ", err)
			}
		}
	}
}
