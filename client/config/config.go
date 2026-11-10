package config

import "net"

type Config struct {
	LocalAddr  net.IP
	ServerAddr net.UDPAddr
	ForwardAll bool
	Seed       [32]byte
}
