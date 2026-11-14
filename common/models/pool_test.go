package models

import (
	"sync"
	"testing"
	"time"
)

func TestPoolConcurrent(t *testing.T) {
	p := CreatePool[uint64]()

	const iterations = 5_000
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			for range iterations {
				idx := p.Acquire(uint32(i))
				time.Sleep(time.Microsecond)
				p.Release(idx)
			}
		})
	}

	wg.Wait()
}

func BenchmarkPool(b *testing.B) {
	b.Run("serial", func(b *testing.B) {
		p := CreatePool[uint64]()
		b.ResetTimer()
		for i := range b.N {
			idx := p.Acquire(uint32(i))
			p.Release(idx)
		}
	})

	b.Run("parallel", func(b *testing.B) {
		p := CreatePool[uint64]()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0 
			for pb.Next() {
				idx := p.Acquire(uint32(i))
				i += 1
				p.Release(idx)
			}
		})
	})
}
