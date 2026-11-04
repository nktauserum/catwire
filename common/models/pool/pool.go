package pool

import (
	"math/bits"
	"sync/atomic"
	"time"
)

type Pool[T any] struct {
	bitmap uint64
	data   [64]T
}

func CreatePool[T any]() *Pool[T] {
	return &Pool[T]{
		bitmap: ^uint64(0),
	}
}

func (p *Pool[T]) Acquire() int {
	backoff := time.Nanosecond
	for {
		bitmap := atomic.LoadUint64(&p.bitmap)
		if bitmap != 0 {
			offset := bits.TrailingZeros64(bitmap)
			if atomic.CompareAndSwapUint64(&p.bitmap, bitmap, bitmap^(1<<offset)) {
				return offset
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

func (p *Pool[T]) Release(index int) {
	if index < 0 || index >= len(p.data) {
		return // wrong index
	}

	atomic.OrUint64(&p.bitmap, uint64(1<<index))
}
