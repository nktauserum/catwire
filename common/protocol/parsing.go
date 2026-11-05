package protocol

import (
	"encoding/binary"
	"unsafe"
)

// JUST CASTING, you should not release the buffer before the packet
func DecodePacket(data []byte) (Packet, bool) {
	var packet Packet
	if len(data) <= HeaderSize {
		return packet, false
	}

	packet.Header = Header{
		Flags:     binary.BigEndian.Uint32(data[0:4]),
		PeerIndex: binary.BigEndian.Uint32(data[4:8]),
		Counter:   binary.BigEndian.Uint64(data[8:16]),
	}
	copy(packet.Header.Nonce[:], data[HeaderSize-32:HeaderSize])

	packet.Payload = unsafe.Slice(&data[HeaderSize], len(data)-HeaderSize)

	return packet, true
}
