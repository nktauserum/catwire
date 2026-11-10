package models

type Pool[T any] struct {
	bitmap uint64
	data   [64]T
}

func CreatePool[T any]() *Pool[T] {
	return &Pool[T]{
		bitmap: ^uint64(0),
	}
}

func (p *Pool[T]) Get(idx uint32) *T {
	return &p.data[idx]
}

func (p *Pool[T]) Acquire() uint32 {
	return pool_acquire(&p.bitmap)
}

func (p *Pool[T]) Release(idx uint32) {
	if idx >= 64 {
		return
	}

	pool_release(&p.bitmap, idx)
}
