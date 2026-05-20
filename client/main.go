package main

import (
	"log"
	"os/exec"
	"net"
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

func main() {
	c := water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams {
			Name: "cw0",
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

	log.Println("Successfully created TUN interface")

	conn, err := net.Dial("udp", serverUDP)
	if err != nil {
		log.Fatalln("error dialing to the server: ", err)
	}
	defer conn.Close()

	log.Printf("Dialing the connection to the server on %s\n", serverUDP)

	go read(tun, conn)
	go write(tun, conn)

	select {}
}

func read(tun *water.Interface, conn net.Conn) {
	buf := make([]byte, 65535)
	p := common.Packet {
		Header: common.Header {
			PacketType: common.DATA,	
			SequenceNumber: 0,
			AdditionalData: 0,
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
		p.Header.SequenceNumber = nextSequenceNumber.Load()
		p.Header.AdditionalData = 0
		p.Payload = buf[:n]

		encodedPacket := common.SendNewPacket(p)
		nextSequenceNumber.Add(1)

		if _, err = conn.Write(encodedPacket); err != nil {
			log.Printf("write: %v", err)
			continue
		}
	}
}

func write(tun *water.Interface, conn net.Conn) {
	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("error reading from %s: %v\n", conn.RemoteAddr(), err)
			continue
		}

		p := common.ReceiveNewPacket(buf[:n])

		if _, err = tun.Write(p.Payload); err != nil {
			log.Printf("Error writing to %s: %v", tun.Name(), err)
			continue
		}
	}

}
