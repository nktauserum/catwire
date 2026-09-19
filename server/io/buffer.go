package io

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	BUFFER_COUNT = 256
	BUFFER_SIZE  = 65535 + 16
)

const (
	IORING_REGISTER_PBUF_RING = 22
	IOSQE_FIXED_FILE          = 1 << 0
	IOSQE_BUFFER_SELECT       = 1 << 4
	IORING_RECV_MULTISHOT     = 1 << 1
	IORING_CQE_F_BUFFER       = 1 << 0
)

type registerBuf struct {
	addr    uint64
	entries uint32
	bgid    uint16
	flags   uint16
	resv    [3]uint64
}

type internalBuffer struct {
	addr uint64
	len  uint32
	bid  uint16 // buffer ID
	tail uint16
}

type internalPool struct {
	ring *internalBuffer
	base unsafe.Pointer
}

func CreateInternalPool(ringFD int) (*internalPool, error) {
	var pool internalPool

	mapSize := (unsafe.Sizeof(internalBuffer{}) + BUFFER_SIZE) * BUFFER_COUNT
	mapped, _, err := syscall.RawSyscall6(syscall.SYS_MMAP, 0, uintptr(mapSize),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_ANONYMOUS|syscall.MAP_PRIVATE,
		0, 0,
	)
	if err != 0 {
		return nil, fmt.Errorf("error mapping the buffer: %v", err)
	}

	pool.ring = (*internalBuffer)(unsafe.Pointer(mapped))
	pool.ring.tail = 0

	pool.base = unsafe.Add(unsafe.Pointer(mapped), unsafe.Sizeof(internalBuffer{})*BUFFER_COUNT)

	reg := registerBuf{
		addr:    uint64(uintptr(unsafe.Pointer(pool.ring))),
		entries: BUFFER_COUNT,
		bgid:    0,
	}

	_, _, errno := syscall.RawSyscall6(
		uintptr(ioUringRegisterSys),
		uintptr(ringFD),
		uintptr(IORING_REGISTER_PBUF_RING),
		uintptr(unsafe.Pointer(&reg)),
		1, // nr_args (!!)
		0, 0,
	)
	if errno != 0 {
		return nil, fmt.Errorf("io_uring_register: %v", errno)
	}

	for i := range BUFFER_COUNT {
		addr := unsafe.Add(pool.base, i*BUFFER_SIZE)

		entry := (*internalBuffer)(unsafe.Add(unsafe.Pointer(pool.ring), uintptr(i)*unsafe.Sizeof(internalBuffer{})))

		entry.addr = uint64(uintptr(addr))
		entry.len = uint32(BUFFER_SIZE)
		entry.bid = uint16(i)
	}

	pool.ring.tail = BUFFER_COUNT

	return &pool, nil
}
