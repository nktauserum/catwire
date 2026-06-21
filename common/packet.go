package common

import (
	"encoding/binary"
)

const (
	DATA uint8 = iota
	HANDSHAKE_INIT
)

/*
	Header - структура фиксированного размера, которая идёт в начале любого catwire-пакета.
	- PacketType: тип данного пакета (данные, рукопожатие)
	- PeerIndex: присвоенный сервером номер данного пира для определения состояния
	- Counter: номер данного пакета, применяемый для шифрования и защиты от повтора
*/

type Header struct {
	PacketType uint8
	PeerIndex  uint64
	Counter    uint64
}

const HeaderSize = 1 + 8 + 8

type Packet struct {
	Header  Header
	Payload []byte
}

func DecodePacket(data []byte) Packet {
	payload := make([]byte, len(data)-HeaderSize)
	copy(payload, data[HeaderSize:])

	return Packet{
		Header: Header{
			PacketType: data[0],
			PeerIndex:  binary.BigEndian.Uint64(data[1:9]),
			Counter:    binary.BigEndian.Uint64(data[9:HeaderSize]),
		},
		Payload: payload,
	}
}

func EncodePacket(p Packet) []byte {
	buf := make([]byte, HeaderSize+len(p.Payload))

	buf[0] = p.Header.PacketType
	binary.BigEndian.PutUint64(buf[1:9], p.Header.PeerIndex)
	binary.BigEndian.PutUint64(buf[9:HeaderSize], p.Header.Counter)
	copy(buf[HeaderSize:], p.Payload)

	return buf
}
