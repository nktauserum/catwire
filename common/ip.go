package common

import (
	"net"
)

func ExtractDestinationIP(packet []byte) string {
	ip := net.IP(packet[16:20])
	return ip.String()
}

func ExtractSourceIP(packet []byte) string {
	ip := net.IP(packet[12:16])
	return ip.String()
}
