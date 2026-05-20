package common

import (
	"encoding/binary"
)

const (
	DATA uint8 = iota
	ACK
	NAK
)

type Header struct {
	PacketType 		uint8
	SequenceNumber 	uint64
	AdditionalData 	uint64
}

const HeaderSize = 1 + 8 + 8

type Packet struct {
	Header 	Header
	Payload []byte
}

func ReceiveNewPacket(data []byte) Packet {
	return Packet {
		Header: Header {
			PacketType: 		data[0],
			SequenceNumber: 	binary.BigEndian.Uint64(data[1:9]),
			AdditionalData:  	binary.BigEndian.Uint64(data[10:HeaderSize]),
		},
		Payload: data[HeaderSize:],
	}
}

func SendNewPacket(p Packet) []byte {
	// using make just for now. then an external buffer will be used
	buf := make([]byte, HeaderSize + len(p.Payload))

	buf[0] = p.Header.PacketType
	binary.BigEndian.PutUint64(buf[1:9], p.Header.SequenceNumber)
	binary.BigEndian.PutUint64(buf[10:HeaderSize], p.Header.AdditionalData)
	copy(buf[HeaderSize:], p.Payload)

	return buf
}
