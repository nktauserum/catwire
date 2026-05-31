package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSendNewPacket(t *testing.T) {
	payload := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	header := Header{
		PacketType: DATA,
		PeerIndex:  0,
		Counter:    0,
	}

	packet := Packet{Payload: payload, Header: header}
	encodedPacket := SendNewPacket(packet)

	t.Logf("packet: \n%#v\n", packet)
	t.Logf("encoded packet: \n%v\n", encodedPacket)

	expectedLength := HeaderSize + len(payload)

	if len(encodedPacket) != expectedLength {
		t.Fatalf("wrong length: expected %v got %v\n", expectedLength, len(encodedPacket))
	}
}

func TestReceiveNewPacket(t *testing.T) {
	payload := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	header := Header{
		PacketType: DATA,
		PeerIndex:  0,
		Counter:    0,
	}
	encodedPacket := SendNewPacket(Packet{Payload: payload, Header: header})
	packet := ReceiveNewPacket(encodedPacket)

	assert.Equal(t, header.PacketType, packet.Header.PacketType, "PacketType don't match")
	assert.Equal(t, header.PeerIndex, packet.Header.PeerIndex, "PeerIndex don't match")
	assert.Equal(t, header.Counter, packet.Header.Counter, "Counter don't match")

	assert.Equal(t, payload, packet.Payload, "Payload don't match")

}
