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

const ipAddr = "10.0.5.1"
const peer = "10.0.5.2"

var remoteAddr atomic.Pointer[net.UDPAddr]
var nextSequenceNumber atomic.Uint64

type State uint8

const (
	StateWaitHandshakeInit State = iota
	StateWaitHandshakeResp
	StateHandshakeFinish
	StateWaiting
	StateTransmit
)

type Session struct {
	s State

	clientPublicKey *ecdh.PublicKey
	secret          []byte

	outgoing        chan []byte
	incoming chan common.Packet

	curve            ecdh.Curve
	serverPrivateKey *ecdh.PrivateKey
	serverPublicKey  *ecdh.PublicKey

	crypto *common.Crypto
}

func (s *Session) HandlePacket(p common.Packet) error {
	switch s.s {
	case StateWaitHandshakeInit:
		if p.Header.PacketType != common.HANDSHAKE_INIT {
			// not matching
			return nil
		}

		var err error
		s.clientPublicKey, err = s.curve.NewPublicKey(p.Payload)
		if err != nil {
			log.Printf("error creating new public key: %v\n", err)
			return err
		}

		s.secret, err = s.serverPrivateKey.ECDH(s.clientPublicKey)
		if err != nil {
			log.Printf("error computing the secret: %v\n", err)
			return err
		}

		log.Printf("secret: %v\n", s.secret)

		p := common.Packet{
			Header: common.Header{
				PacketType: common.HANDSHAKE_INIT,
				PeerIndex:  0, // maybe here we'll set the peer index
				Counter:    nextSequenceNumber.Load(),
			},
			Payload: s.serverPublicKey.Bytes(),
		}

		enc := common.EncodePacket(p)
		s.outgoing <- enc

		s.crypto, err = common.NewCrypto(s.secret)
		if err != nil {
			log.Printf("error creating crypto: %v\n", err)
			return err
		}


		s.s = StateTransmit

	case StateTransmit:


	default:
		return nil
	}

	return nil
}

func main() {
	c := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: "cw1",
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

	// init crypto
	curve := ecdh.X25519()
	serverPrivateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("error generating server private key: %v\n", err)
	}
	serverPublicKey := serverPrivateKey.PublicKey()

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

	s := Session{
		s: StateWaitHandshakeInit,

		clientPublicKey: nil,
		secret:          nil,

		outgoing: outgoing,
		incoming: incoming,

		curve:            curve,
		serverPrivateKey: serverPrivateKey,
		serverPublicKey:  serverPublicKey,

		crypto: nil,
	}

	go s.sendUDP(conn)
	go s.listenUDP(tun, conn)
	go s.listenTUN(tun)

	for p := range incoming {
		s.HandlePacket(p)
	}
}

func (s *Session) listenUDP(tun *water.Interface, conn *net.UDPConn) {
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
			decrypted, err := s.crypto.Decrypt(p.Payload)
			if err != nil {
				log.Println("listenUDP: decrypt: ", err)
				continue
			}

			if _, err = tun.Write(decrypted); err != nil {
				log.Println("read: ", err)
			}
			continue
		}

		s.incoming <- p
	}
}

func (s *Session) listenTUN(tun *water.Interface) {
	buf := make([]byte, 65535)

	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Println("write: ", err)
			continue
		}

		if s.crypto == nil { continue }

		encryptedData, err := s.crypto.Encrypt(buf[:n])
		if err != nil {
			log.Printf("listenTUN: encrypt: %v\n", err)
			continue
		}

		p := common.Packet{
			Header: common.Header{
				PacketType: common.DATA,
				PeerIndex:  0,
				Counter:    nextSequenceNumber.Load(),
			},
			Payload: encryptedData,
		}

		encodedPacket := common.EncodePacket(p)
		nextSequenceNumber.Add(1)

		s.outgoing <- encodedPacket
	}
}

func (s *Session) sendUDP(conn *net.UDPConn) {
	for packet := range s.outgoing {
		if clientAddr := remoteAddr.Load(); clientAddr != nil {
			if _, err := conn.WriteToUDP(packet, clientAddr); err != nil {
				log.Println("write: ", err)
			}
		}
	}
}
