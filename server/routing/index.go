package routing

import (
	"encoding/binary"
	"net"
)

const subnetMask = (0xFFFFFFFF << (32 - 24)) & 0xFFFFFFFF // 24 as CIDR notation (0xFFFFFF00)
const ServerAddr = "10.0.5.1"
var ServerAddrInt, _ = IPAsInteger(net.ParseIP(ServerAddr))
var subnetAddr = getIPSubnet(ServerAddrInt, subnetMask)

func IPAsInteger(addr net.IP) (uint32, bool) {
	ipv4 := addr.To4()
	if ipv4 != nil {
		return binary.BigEndian.Uint32(ipv4), false
	}
	return 0, true
}

func ExtractIndex(addr uint32) (uint32, bool) {
	idx_base := addr >> 24
	return idx_base - 2, idx_base >= 2
}

func ExtractDestinationIP(packet []byte) uint32 {
	ip := net.IP(packet[16:20])
	return binary.BigEndian.Uint32(ip)
}

func ExtractSourceIP(packet []byte) uint32 {
	ip := net.IP(packet[12:16])
	return binary.BigEndian.Uint32(ip)
}

func getIPSubnet(ip uint32, mask uint32) uint32 {
	return ip & mask
}

func IPInLocalSubnet(ip uint32) bool {
	return getIPSubnet(ip, subnetMask) == subnetAddr
}

func IsInternal(packet []byte) bool {
	destIP := ExtractDestinationIP(packet)
	if IPInLocalSubnet(destIP) && destIP != ServerAddrInt {
		return true
	}

	return false
}
