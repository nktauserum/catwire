package main

import (
	"sync"
	"testing"
	"time"
)

func TestEventTableOperations(t *testing.T) {
	table := CreateEventTable()

	var wg sync.WaitGroup
	start := time.Now()

	for range 100 {
		wg.Go(func() {
			for range 1000 {
				idx := table.Acquire()
				time.Sleep(time.Microsecond)
				table.Release(idx)
			}
		})
	}

	wg.Wait()
	since := time.Since(start)
	t.Logf("Total time: %v\n", since)
	t.Logf("Operations: 100k, avg: %v/op\n", (since-time.Microsecond*1000)/100000/2)
}
