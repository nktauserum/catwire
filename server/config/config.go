package config

import "net"

type Config struct {
	Address net.UDPAddr
	Seed    [32]byte
	Clients []Client
}

type Client struct {
	Name      string
	LocalAddr net.IP
	PublicKey [32]byte
}
