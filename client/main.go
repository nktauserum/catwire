package main

import (
	"log"
	"os/exec"
	"net"

	"github.com/songgao/water"
)

const (
	ipAddr    = "10.0.5.2" 
	peer      = "10.0.5.1"
	serverUDP = "192.168.1.2:55635"
)

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

	conn, err := net.Dial("udp", serverUDP)
	if err != nil {
		log.Fatalln("error dialing to the server: ", err)
	}
	defer conn.Close()

	go read(tun, conn)
	go write(tun, conn)

	select {}
}

func read(tun *water.Interface, conn net.Conn) {
	buf := make([]byte, 2048)
	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Printf("read: %v\n", err)
			continue
		}

		if _, err = conn.Write(buf[:n]); err != nil {
			log.Printf("write: %v", err)
			continue
		}
	}
}

func write(tun *water.Interface, conn net.Conn) {
	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("error reading from %s: %v\n", conn.RemoteAddr(), err)
			continue
		}

		if _, err = tun.Write(buf[:n]); err != nil {
			log.Printf("Error writing to %s: %v", tun.Name(), err)
			continue
		}
	}

}
