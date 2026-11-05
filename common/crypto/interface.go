// Package crypto provides an interface to libsodium
package crypto

/*
#cgo LDFLAGS: -lsodium
#include <sodium.h>
*/
import "C"

func Init() bool {
	return C.sodium_init() >= 0
}
