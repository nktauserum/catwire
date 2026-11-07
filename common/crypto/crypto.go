package crypto

/*
#include <sodium.h>
*/
import "C"
import (
	"unsafe"

	"github.com/nktauserum/catwire/common/protocol"
)

func Decrypt(dst, src []byte, key [32]byte) ([]byte, bool) {
	var size uint64
	ret := C.crypto_aead_aegis256_decrypt(
		(*C.uchar)(unsafe.Pointer(&dst[0])), (*C.ulonglong)(&size), (*C.uchar)(unsafe.Pointer(uintptr(0))),
		(*C.uchar)(unsafe.Pointer(&src[protocol.HeaderSize])), (C.ulonglong)(len(src)-protocol.HeaderSize),
		(*C.uchar)(unsafe.Pointer(&src[0])), (C.ulonglong)(protocol.HeaderSize),
		(*C.uchar)(unsafe.Pointer(&src[protocol.HeaderSize-32])), (*C.uchar)(&key[0]),
	) // TODO: add fallback

	return dst[:size], ret < 0
}

func Encrypt(dst, src []byte, key [32]byte) ([]byte, bool) {
	var size uint64
	ret := C.crypto_aead_aegis256_encrypt(
		(*C.uchar)(unsafe.Pointer(&dst[0])), (*C.ulonglong)(&size),
		(*C.uchar)(unsafe.Pointer(&src[protocol.HeaderSize])), (C.ulonglong)(len(src)-protocol.HeaderSize),
		(*C.uchar)(unsafe.Pointer(&src[0])), (C.ulonglong)(protocol.HeaderSize),
		(*C.uchar)(unsafe.Pointer(uintptr(0))),
		(*C.uchar)(unsafe.Pointer(&src[protocol.HeaderSize-32])), (*C.uchar)(&key[0]),
	)

	return dst[:size], ret < 0
}
