package io

import (
	"net"
	"testing"
)

func TestMultishot(t *testing.T) {
	ring, err := NewRingDefault()
	if err != nil {
		t.Fatalf("Error creating new ring: %v\n", err.Error())
	}

	pool, err := createInternalPool(ring.ringFd)
	if err != nil {
		t.Fatalf("Error creating new pool: %v\n", err.Error())
	}

	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:45230")
	if err != nil {
		t.Fatalf("Error resolving udp addr: %v\n", err.Error())
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		t.Fatalf("Error listening udp: %v\n", err.Error())
	}

	f, err := conn.File()
	if err != nil {
		t.Fatalf("Error copying file: %v\n", err.Error())
	}

	ret, errno := ring.submitMultishot(pool, int32(f.Fd()))
	if errno != 0 {
		t.Fatalf("Error multishot: %v\n", errno.Error())
	}

	t.Logf("Return: %v\n", ret)
}
