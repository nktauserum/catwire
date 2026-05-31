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
	PeerIndex  uint16
	Counter    uint64
}

const HeaderSize = 1 + 2 + 8

type Packet struct {
	Header  Header
	Payload []byte
}

func DecodePacket(data []byte) Packet {
	return Packet{
		Header: Header{
			PacketType: data[0],
			PeerIndex:  binary.BigEndian.Uint16(data[1:3]),
			Counter:    binary.BigEndian.Uint64(data[3:HeaderSize]),
		},
		Payload: data[HeaderSize:],
	}
}

func EncodePacket(p Packet) []byte {
	buf := make([]byte, HeaderSize+len(p.Payload))

	buf[0] = p.Header.PacketType
	binary.BigEndian.PutUint16(buf[1:3], p.Header.PeerIndex)
	binary.BigEndian.PutUint64(buf[3:HeaderSize], p.Header.Counter)
	copy(buf[HeaderSize:], p.Payload)

	return buf
}
