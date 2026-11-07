package routing

import (
	"encoding/binary"
	"net"
)

func IPAsInteger(addr net.IP) (uint32, bool) {
	ipv4 := addr.To4()
	if ipv4 != nil {
		return binary.BigEndian.Uint32(addr), false
	}
	return 0, true
}

func ExtractIndex(addr uint32) (uint32, bool) {
	idx_base := (addr >> 24) & 0xFF
	return idx_base - 2, idx_base < 2
}
