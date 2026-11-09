package config

import "net"

type Config struct {
	ServerAddr net.UDPAddr
	Seed       [32]byte
}
