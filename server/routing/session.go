package routing

/*
#cgo LDFLAGS: -lsodium -static
#include <sodium.h>
*/
import "C"
import (
	"net"
	"unsafe"

	"github.com/nktauserum/catwire/common/protocol"
)

type Session struct {
	Active                             bool
	PublicKey, ReceiveKey, TransmitKey [32]byte
	PublicAddr                         net.UDPAddr
	LocalAddr                          net.IP
	ReceiveCounter, TransmitCounter    uint64 // TODO: move to the routing table
}

func (s *Session) Decrypt(dst, src []byte) ([]byte, bool) {
	var size uint64
	ret := C.crypto_aead_aegis256_decrypt(
		(*C.uchar)(unsafe.Pointer(&dst[0])), (*C.ulonglong)(&size), (*C.uchar)(unsafe.Pointer(uintptr(0))),
		(*C.uchar)(unsafe.Pointer(&src[protocol.HeaderSize])), (C.ulonglong)(len(src)-protocol.HeaderSize),
		(*C.uchar)(unsafe.Pointer(&src[0])), (C.ulonglong)(protocol.HeaderSize),
		(*C.uchar)(unsafe.Pointer(&src[protocol.HeaderSize-32])), (*C.uchar)(&s.ReceiveKey[0]),
	) // TODO: add fallback

	return dst[:size], ret < 0
}
