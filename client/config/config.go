package config

import "net"

type Config struct {
	LocalAddr  net.IP
	ServerAddr net.UDPAddr
	Seed       [32]byte
}
