// Package protocol describes catwire protocol itself
package protocol

const (
	F_DATA      uint32 = 1 << 0
	F_HANDSHAKE uint32 = 1 << 1
	F_AEGIS256  uint32 = 1 << 2

	// TODO: ChaCha-PoLy-x... fallback
)

type Header struct {
	Flags     uint32
	PeerIndex uint32
	Counter   uint64
	Nonce     [32]byte
}

const HeaderSize int = 4 + 4 + 8 + 32

type Packet struct {
	Header  Header
	Payload []byte
}
