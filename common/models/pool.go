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

func (p *Pool[T]) Acquire() uint32 {
	return pool_acquire(&p.bitmap)
}

func (p *Pool[T]) Release(idx uint32) {
	// if index < 0 || index >= len(p.data) {
	// 	return // wrong index
	// }
	//
	// atomic.OrUint64(&p.bitmap, uint64(1<<index))
	pool_release(&p.bitmap, idx)
}
