package routing

import (
	"encoding/binary"
	"net"
)

func IPAsInteger(addr net.IP) (uint32, bool) {
	ipv4 := addr.To4()
	if ipv4 != nil {
		return binary.LittleEndian.Uint32(ipv4), false
	}
	return 0, true
}

func ExtractIndex(addr uint32) (uint32, bool) {
	idx_base := addr >> 24
	return idx_base - 2, idx_base >= 2
}

func ExtractDestinationIP(packet []byte) uint32 {
	ip := net.IP(packet[16:20])
	return binary.LittleEndian.Uint32(ip)
}

