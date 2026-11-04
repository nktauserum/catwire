//go:build !amd64

package models

import (
	"math/bits"
	"sync/atomic"
	"time"
)

func pool_acquire(bitmap *uint64) uint32 {
	backoff := time.Nanosecond
	for {
		b := atomic.LoadUint64(bitmap)
		if b != 0 {
			offset := bits.TrailingZeros64(b)
			if atomic.CompareAndSwapUint64(bitmap, b, b^(1<<offset)) {
				return uint32(offset)
			}

			backoff = time.Nanosecond
			continue
		}

		time.Sleep(backoff)
		if backoff < time.Millisecond {
			backoff *= 2
		}
	}
}
