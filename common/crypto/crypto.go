package crypto

/*
#include <sodium.h>
*/
import "C"
import (
	"unsafe"
)

func Decrypt(dst, src, ad, nonce []byte, key [32]byte) (uint64, bool) {
	var size uint64
	ret := C.crypto_aead_aegis256_decrypt(
		(*C.uchar)(unsafe.Pointer(&dst[0])), (*C.ulonglong)(&size), (*C.uchar)(unsafe.Pointer(uintptr(0))), // dst
		(*C.uchar)(unsafe.Pointer(&src[0])), (C.ulonglong)(len(src)), // src
		(*C.uchar)(unsafe.Pointer(&ad[0])), (C.ulonglong)(len(ad)), // ad
		(*C.uchar)(unsafe.Pointer(&nonce[0])), (*C.uchar)(&key[0]), // nonce and key
	) // TODO: add fallback

	return size, ret < 0
}

func Encrypt(dst, src, ad, nonce []byte, key [32]byte) (uint64, bool) {
	var size uint64
	ret := C.crypto_aead_aegis256_encrypt(
		(*C.uchar)(unsafe.Pointer(&dst[0])), (*C.ulonglong)(&size), // dst
		(*C.uchar)(unsafe.Pointer(&src[0])), (C.ulonglong)(len(src)), // src
		(*C.uchar)(unsafe.Pointer(&ad[0])), (C.ulonglong)(len(ad)), // ad
		(*C.uchar)(unsafe.Pointer(uintptr(0))),                     // nsec is never used
		(*C.uchar)(unsafe.Pointer(&nonce[0])), (*C.uchar)(&key[0]), // nonce and key
	)

	return size, ret < 0
}
