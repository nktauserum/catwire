package common

import (
	"encoding/binary"
	"net"
)

func ExtractDestinationIP(packet []byte) uint32 {
	ip := net.IP(packet[16:20])
	return binary.BigEndian.Uint32(ip)
}

func ExtractSourceIP(packet []byte) uint32 {
	ip := net.IP(packet[12:16])
	return binary.BigEndian.Uint32(ip)
}
