package routing

import (
	"encoding/binary"
	"net"
)

func ExtractIndex(addr net.IP) (uint32, bool) {
	ipv4 := addr.To4()
	if ipv4 != nil {
		return binary.BigEndian.Uint32(addr), false
	}
	return 0, true
}
