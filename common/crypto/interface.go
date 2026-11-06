// Package crypto provides an interface to libsodium
package crypto

/*
#cgo LDFLAGS: -lsodium -static
#include <sodium.h>
*/
import "C"
import "unsafe"

// var _ = func() bool {
// 	return C.sodium_init() >= 0
// }()

type Cipher struct {
	PublicKey, privateKey [32]byte
}

func RandomBytes(buf []byte) {
	C.randombytes_buf(unsafe.Pointer(unsafe.SliceData(buf)), C.size_t(len(buf)))
}
