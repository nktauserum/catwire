package routing

import (
	"encoding/binary"
)

func ExtractDestinationIP(packet []byte) uint32 {
	return binary.LittleEndian.Uint32(packet[16:20])
}

func ExtractIndex(ip uint32) (idx uint32, ok bool) {
	host := (ip >> 24) & 0xFF
	if host < 2 {
		return 0, false
	}
	return host - 2, true
}

func IsInternal(packet []byte) bool {
	ip := ExtractDestinationIP(packet)
	if ip&0x00FFFFFF == 0x0005000A && ip != 0x0105000A {
		return true
	}
	return false
}
