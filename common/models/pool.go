package models

import (
	"sync/atomic"
	"time"
)

func ctzll(uint64) uint32 

type Pool[T any] struct {
	bitmap uint64
	data   [64]T
}

func CreatePool[T any]() *Pool[T] {
	return &Pool[T]{
		bitmap: ^uint64(0),
	}
}

func (p *Pool[T]) Acquire() uint32 {
	backoff := time.Nanosecond
	for {
		bitmap := atomic.LoadUint64(&p.bitmap)
		if bitmap != 0 {
			offset := ctzll(bitmap)
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
