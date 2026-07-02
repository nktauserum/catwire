package common

import (
	"encoding/binary"
	"fmt"
)

const (
	DATA uint8 = iota
	HANDSHAKE_INIT
)

var ErrTooShortPacket error = fmt.Errorf("the provided packet was smaller than 17 bytes")

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

func DecodePacket(data []byte) (Packet, error) {
	if len(data) < 17 {
		return Packet{}, ErrTooShortPacket
	}

	return Packet{
		Header: Header{
			PacketType: data[0],
			PeerIndex:  binary.BigEndian.Uint64(data[1:9]),
			Counter:    binary.BigEndian.Uint64(data[9:HeaderSize]),
		},
		Payload: data[HeaderSize:],
	}, nil
}

func EncodePacket(p Packet) []byte {
	buf := make([]byte, HeaderSize+len(p.Payload))

	buf[0] = p.Header.PacketType
	binary.BigEndian.PutUint64(buf[1:9], p.Header.PeerIndex)
	binary.BigEndian.PutUint64(buf[9:HeaderSize], p.Header.Counter)
	copy(buf[HeaderSize:], p.Payload)

	return buf
}
