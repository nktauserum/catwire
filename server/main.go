package main

import (
	"fmt"
	"math/bits"
	"sync/atomic"
	"time"
)

const maxUint64 = ^uint64(0)

type Event struct {
	Buffer [65535 + 16]byte
}

type EventTable struct {
	events [64]Event
	bitmap uint64
}

func CreateEventTable() *EventTable {
	return &EventTable{
		bitmap: maxUint64,
	}
}

func (t *EventTable) Acquire() int {
	backoff := time.Nanosecond
	for {
		bitmap := atomic.LoadUint64(&t.bitmap)
		if bitmap != 0 {
			offset := bits.TrailingZeros64(bitmap)
			if atomic.CompareAndSwapUint64(&t.bitmap, bitmap, bitmap^(1<<offset)) {
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

func (t *EventTable) Release(index int) {
	if index < 0 || index >= len(t.events) {
		return // wrong index
	}

	atomic.OrUint64(&t.bitmap, uint64(1<<index))
}

func main() {
	table := CreateEventTable()

	for i := range 5 {
		acquire := time.Now()
		idx := table.Acquire()
		fmt.Printf("Acquire #%v: got index %v on time %v\n", i, idx, time.Since(acquire))

		release := time.Now()
		table.Release(idx)
		fmt.Printf("Release #%v: time %v\n", i, time.Since(release))
	}
}
