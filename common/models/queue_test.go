package models

import "testing"

func TestQueue(t *testing.T) {
    q := CreateQueue[uint64]()
    const iterations = 5_000

    go func() {
        for i := range iterations {
            got, ok := q.Receive()
            if !ok || got != uint64(i) {
                t.Errorf("Receive() = (%d, %v), want (%d, true)", got, ok, i)
            }
        }
    }()

    for i := range iterations {
        if !q.Send(uint64(i)) {
            t.Fatal("Send() = false")
        }
    }
}
