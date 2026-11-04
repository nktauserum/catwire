//go:build !amd64

package models

import "math/bits"

func ctzll(n uint64) uint32 {
	return uint32(bits.TrailingZeros64(n))
}
