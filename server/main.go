package main

import (
	"log"
	"os/exec"
	"sync/atomic"
	"net"

	"github.com/songgao/water"
)

const ipAddr = "10.0.6.1"
const peer = "10.0.6.2"
var remoteAddr atomic.Pointer[net.UDPAddr] 

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

	addr, err := net.ResolveUDPAddr("udp", ":55635")
	if err != nil {
		log.Fatalf("error resolving udp addr: %v\n", err)
	}

	conn, err := net.ListenUDP("udp", addr)	
	if err != nil {
		log.Fatalf("error listening: %v\n", err)
	}
	defer conn.Close()

	go read(tun, conn)
	go write(tun, conn)

	select {}
}


func read(tun *water.Interface, conn *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("read: ", err)
			continue
		}

		remoteAddr.Store(clientAddr)

		if _, err = tun.Write(buf[:n]); err != nil {
			log.Println("read: ", err)
		}
	}
}

func write(tun *water.Interface, conn *net.UDPConn) {
	buf := make([]byte, 2048);

	for {
		n, err := tun.Read(buf)
		if err != nil {
			log.Println("write: ", err)
			continue
		}

		if clientAddr := remoteAddr.Load(); clientAddr != nil {
			if _, err := conn.WriteToUDP(buf[:n], clientAddr); err != nil {
				log.Println("write: ", err)
			}
		}
	}
}
