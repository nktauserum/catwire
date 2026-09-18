package io

import (
	"os"
	"testing"
	"unsafe"
)

const wanted = "Hello World!"

func TestSetup(t *testing.T) {
	_, err := NewRingDefault()
	if err != nil {
		t.Fatalf("Error setup ring: %v\n", err.Error())
	}
}

func TestSubmitToSQ(t *testing.T) {
	r, err := NewRingDefault()
	if err != nil {
		t.Fatalf("Error setup ring: %v\n", err.Error())
	}

	f, err := os.Create("./example.txt")
	if err != nil {
		t.Fatalf("error creating file: %v\n", err)
	}
	defer f.Close()
	defer os.Remove("./example.txt")

	txt := []byte(wanted)

	r.submitToSQ(opWrite, int32(f.Fd()), uintptr(unsafe.Pointer(&txt[0])), uint32(len(txt)), 0)

	enter(r.ringFd, 0, 1, uint32(ioringEnterGetEvents))

	_, ok := r.readFromCQ()
	if !ok {
		t.Fail()
	}

	buf := make([]byte, 1024)
	r.submitToSQ(opRead, int32(f.Fd()), uintptr(unsafe.Pointer(&buf[0])), 1024, 0)

	enter(r.ringFd, 0, 1, uint32(ioringEnterGetEvents))

	if string(buf[:len(wanted)]) != wanted {
		t.Fatalf("Error: got %v, wanted %v\n", string(buf), wanted)
	}
}
