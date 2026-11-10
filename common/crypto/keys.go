package crypto

/*
#cgo LDFLAGS: -lsodium -static
#include <sodium.h>
*/
import "C"
import "unsafe"

func CreateCipher(seed [32]byte) (*Cipher, bool) {
	var c Cipher
	ret := C.crypto_kx_seed_keypair(
		(*C.uchar)(unsafe.Pointer(&c.PublicKey)),
		(*C.uchar)(unsafe.Pointer(&c.privateKey)),
		(*C.uchar)(unsafe.Pointer(&seed)),
	)

	return &c, ret < 0
}

func (c *Cipher) GenerateServerKeys(key []byte) ([32]byte, [32]byte, bool) {
	var rx_key, tx_key [32]byte

	ret := C.crypto_kx_server_session_keys(
		(*C.uchar)(unsafe.Pointer(&rx_key)),
		(*C.uchar)(unsafe.Pointer(&tx_key)),
		(*C.uchar)(unsafe.Pointer(&c.PublicKey)),
		(*C.uchar)(unsafe.Pointer(&c.privateKey)),
		(*C.uchar)(unsafe.Pointer(&key[0])),
	)

	return rx_key, tx_key, ret < 0
}

func (c *Cipher) GenerateClientKeys(key []byte) ([32]byte, [32]byte, bool) {
	var rx_key, tx_key [32]byte

	ret := C.crypto_kx_client_session_keys(
		(*C.uchar)(unsafe.Pointer(&rx_key)),
		(*C.uchar)(unsafe.Pointer(&tx_key)),
		(*C.uchar)(unsafe.Pointer(&c.PublicKey)),
		(*C.uchar)(unsafe.Pointer(&c.privateKey)),
		(*C.uchar)(unsafe.Pointer(&key[0])),
	)

	return rx_key, tx_key, ret < 0
}
