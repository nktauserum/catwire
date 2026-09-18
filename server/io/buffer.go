package io

import (
	"fmt"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	BUFFER_COUNT = 256
	BUFFER_SIZE  = unsafe.Sizeof(Buffer{})
)

const (
	IORING_REGISTER_PBUF_RING = 22
	IORING_OP_RECVMSG         = 13
	IOSQE_FIXED_FILE          = 1 << 0
	IOSQE_BUFFER_SELECT       = 1 << 4
	IOSQE_MULTISHOT           = 1 << 3
	IORING_CQE_F_BUFFER       = 1 << 0
)

type Buffer struct {
	data [65535 + 16]byte
}

type bufferRing struct {
	tail uint32
	_    uint32
	buf  [BUFFER_COUNT]internalBuffer
}

type registerBuf struct {
	ring   uint64
	rindex uint32
	resv   uint16
	nrings uint16
}

type internalBuffer struct {
	addr uint64
	len  uint32
	bid  uint16 // buffer ID
}

type internalPool struct {
	data    [BUFFER_SIZE * BUFFER_COUNT]byte
	ring    *bufferRing
	headers [BUFFER_COUNT]syscall.Msghdr
	addrs   [BUFFER_COUNT]syscall.RawSockaddrInet4

	nextidx uint32
	ringidx uint32
}

func createInternalPool() (*internalPool, error) {
	pool := new(internalPool)
	pool.ring = new(bufferRing)

	for i := range BUFFER_COUNT {
		addr := unsafe.Add(unsafe.Pointer(&pool.data[0]), i*int(BUFFER_SIZE))
		pos := atomic.LoadUint32((&pool.ring.tail)) % BUFFER_COUNT

		entry := &pool.ring.buf[pos]

		entry.addr = uint64(uintptr(addr))
		entry.len = uint32(BUFFER_SIZE)
		entry.bid = uint16(i)

		atomic.AddUint32(&pool.ring.tail, 1)
	}

	return pool, nil
}

func (p *internalPool) register(ringFD int) error {
	reg := registerBuf{
		ring:   uint64(uintptr(unsafe.Pointer(p.ring))),
		nrings: 1,
	}

	_, _, errno := syscall.RawSyscall(
		uintptr(ioUringRegisterSys),
		uintptr(ringFD),
		uintptr(IORING_REGISTER_PBUF_RING),
		uintptr(unsafe.Pointer(&reg)),
	)
	if errno != 0 {
		return fmt.Errorf("io_uring_register: %v", errno)
	}

	p.ringidx = reg.rindex

	return nil
}
