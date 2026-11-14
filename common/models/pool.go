package models

const poolSetCapacity = 32

type Pool[T any] struct {
	set [poolSetCapacity]internalPool[T]
}

func CreatePool[T any]() *Pool[T] {
	p := &Pool[T]{}
	for i := range poolSetCapacity {
		p.set[i].bitmap = ^uint64(0)
	}
	return p
}

func (p *Pool[T]) Acquire(hint uint32) uint32 {
	h := hint%poolSetCapacity
	idx := p.set[h].acquire()

	return idx | (h << 16)
}

func (p *Pool[T]) Release(idx uint32) {
	p.set[idx >> 16].release(idx & 0x0000FFFF)
}

type internalPool[T any] struct {
	data   [64]T
	bitmap uint64
}

func createInternalPool[T any]() *internalPool[T] {
	return &internalPool[T]{
		bitmap: ^uint64(0),
	}
}

func (p *internalPool[T]) acquire() uint32 {
	return pool_acquire(&p.bitmap)
}

func (p *internalPool[T]) release(idx uint32) {
	if idx >= 64 {
		return
	}

	pool_release(&p.bitmap, idx)
}
