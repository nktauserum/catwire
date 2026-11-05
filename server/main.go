package main

import (
	"fmt"
	"log"

	"github.com/nktauserum/catwire/server/config"
)

func main() {
	c, err := config.ParseConfig("server/config/config.example.ini")
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Printf("%#v\n", c)
	// addr, err := net.ResolveUDPAddr("udp", ":55352")
	// if err != nil {
	// 	log.Fatalf("error resolving udp addr: %v\n", err)
	// }
	//
	// conn, err := net.ListenUDP("udp", addr)
	// if err != nil {
	// 	log.Fatalf("error listening: %v\n", err)
	// }
	// defer conn.Close()
	//
	// _, err = tun.CreateTUN("cw0")
	// if err != nil {
	// 	log.Fatalf("error creating TUN: %v\n", err)
	// }
}
