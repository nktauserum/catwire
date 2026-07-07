package main

import (
	"crypto/ecdh"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/nktauserum/catwire/common"
	"github.com/nktauserum/catwire/server/config"
	"github.com/nktauserum/catwire/server/routing"
	"github.com/songgao/water"
)

const ipAddr = "10.0.5.1"

type Server struct {
	IndexLookupTable routing.PeerIndices
	AllowedIPs       map[string]uint32
	IPLookupTable    routing.PeerRouting

	curve            ecdh.Curve
	serverPrivateKey *ecdh.PrivateKey
	serverPublicKey  *ecdh.PublicKey

	outgoing chan []byte

	conn *net.UDPConn
}

type Task struct {
	Data       *[]byte
	ClientAddr *net.UDPAddr
}

func (server *Server) listenUDP() {
	buf := make([]byte, 65535)
	pool := sync.Pool{
		New: func() any {
			b := make([]byte, 65535)
			return &b
		},
	}
	ch := make(chan *Task, 1024)

	workersCount := 32

	for range workersCount {
		go func() {
			for t := range ch {
				p, err := common.DecodePacket(*t.Data)
				if err != nil {
					pool.Put(t.Data)
					continue
				}

				if p.Header.PacketType == common.DATA && p.Header.PeerIndex != 0 {
					session, err := server.IndexLookupTable.Load(p.Header.PeerIndex)
					if err != nil {
						pool.Put(t.Data)
						continue
					}

					session.Incoming(p, t.ClientAddr)
					pool.Put(t.Data)
					continue
				}

				// handshake
				if p.Header.PacketType != common.HANDSHAKE_INIT {
					pool.Put(t.Data)
					continue
				}

				key := base64.StdEncoding.EncodeToString(p.Payload)
				clientIP, exists := server.AllowedIPs[key]
				if exists {
					s := routing.NewSession(server.outgoing, server.conn, t.ClientAddr, &server.IPLookupTable)

					var err error
					clientPublicKey, err := server.curve.NewPublicKey(p.Payload)
					if err != nil {
						log.Printf("error creating new public key: %v\n", err)
						pool.Put(t.Data)
						continue
					}

					secret, err := server.serverPrivateKey.ECDH(clientPublicKey)
					if err != nil {
						log.Printf("error computing the secret: %v\n", err)
						pool.Put(t.Data)
						continue
					}

					log.Printf("The shared secret for %v was computed!\n", t.ClientAddr)

					crypto, err := common.NewCrypto(secret)
					if err != nil {
						log.Printf("error creating crypto: %v\n", err)
						pool.Put(t.Data)
						continue
					}

					idx := server.IndexLookupTable.Store(key, s)
					server.IPLookupTable.Store(clientIP, s)

					s.InitSession(idx, clientPublicKey, crypto)

					resp := common.Packet{
						Header: common.Header{
							PacketType: common.HANDSHAKE_INIT,
							PeerIndex:  idx,
							Counter:    s.Counter.Add(1) - 1,
						},
						Payload: server.serverPublicKey.Bytes(),
					}

					enc := common.EncodePacket(resp)

					s.Send(enc) // вызываем внутреннюю функцию Session для отправки байтов сразу в UDP
				}

				pool.Put(t.Data)
			}
		}()
	}

	for {
		n, clientAddr, err := server.conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("read: ", err)
			continue
		}

		data := pool.Get().(*[]byte)
		*data = (*data)[:n]
		copy(*data, buf[:n])

		ch <- &Task{Data: data, ClientAddr: clientAddr}
	}
}

func (server *Server) listenTUN(tun *water.Interface) {
	buf := make([]byte, 65535)
	pool := sync.Pool{
		New: func() any {
			b := make([]byte, 65535)
			return &b
		},
	}
	ch := make(chan *[]byte, 1024)

	workersCount := 32
	for range workersCount {
		go func() {
			for data := range ch {
				destIP := common.ExtractDestinationIP(*data)
				session := server.IPLookupTable.Load(destIP)

				if session == nil {
					pool.Put(data)
					continue
				}

				session.Outgoing(*data)
				pool.Put(data)
			}
		}()
	}

	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Println("write: ", err)
			continue
		}

		data := pool.Get().(*[]byte)
		*data = (*data)[:n]
		copy(*data, buf[:n])

		ch <- data
	}
}

func sendTUN(tun *water.Interface, outgoing chan []byte) {
	for data := range outgoing {
		if _, err := tun.Write(data); err != nil {
			log.Println("sendTUN: ", err)
		}
	}
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Path to config")
	flag.Parse()

	if configPath == "" {
		fmt.Println("Please provide a relevant config. For more info see --help.")
		os.Exit(1)
	}

	config, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("error parsing config: %v\n", err)
	}

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
		{"ip", "addr", "add", ipAddr + "/24", "dev", tun.Name()},
		{"ip", "link", "set", "dev", tun.Name(), "mtu", "1420"},
		{"iptables", "-t", "nat", "-A", "POSTROUTING", "-o", "eth0", "-j", "MASQUERADE"},
		{"iptables", "-A", "FORWARD", "-i", tun.Name(), "-j", "ACCEPT"},
		{"iptables", "-A", "FORWARD", "-o", tun.Name(), "-j", "ACCEPT"},
		{"sysctl", "-w", "net.ipv4.ip_forward=1"},
	}

	for _, cmd := range cmds {
		out, err := exec.Command(cmd[0], cmd[1:]...).CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run %v: %v, output: %s", cmd, err, string(out))
		}
	}

	// init crypto
	curve := ecdh.X25519()
	privKeyBytes, err := base64.StdEncoding.DecodeString(config.PrivateKey)
	if err != nil {
		log.Fatalln("decode private key:", err)
	}

	serverPrivateKey, err := curve.NewPrivateKey(privKeyBytes)
	if err != nil {
		log.Fatalf("error generating client private key: %v\n", err)
	}
	serverPublicKey := serverPrivateKey.PublicKey()

	addr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(config.ListenPort))
	if err != nil {
		log.Fatalf("error resolving udp addr: %v\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	outgoing := make(chan []byte, 1024)

	allowedIPs := make(map[string]uint32)
	for key, ip := range config.AllowedIPs {
		allowedIPs[key] = common.IpAsInteger(ip)
	}

	s := Server{
		conn:     conn,
		outgoing: outgoing,

		curve:            curve,
		serverPrivateKey: serverPrivateKey,
		serverPublicKey:  serverPublicKey,

		IPLookupTable:    routing.NewPeerRouting(),
		IndexLookupTable: routing.NewPeerIndices(len(allowedIPs)),
		AllowedIPs:       allowedIPs,
	}

	go sendTUN(tun, outgoing)
	go s.listenTUN(tun)

	log.Printf("Listening on :%d\n", config.ListenPort)

	s.listenUDP()
}
