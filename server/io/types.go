package io

import (
	"unsafe"
)

type (
	SQE struct {
		opcode   uint8  /* type of operation for this sqe */
		flags    uint8  /* IOSQE_ flags */
		ioprio   uint16 /* ioprio for the request */
		fd       int32  /* file descriptor to do IO on */
		off      uint64 /* offset into file */
		addr     uint64 /* pointer to buffer or iovecs */
		len      uint32 /* buffer size or number of iovecs */
		sqeFlags uint32
		userData uint64 /* data to be passed back at completion time */
		pad      [3]uint64
	}

	sQueue struct {
		khead        *uint32
		ktail        *uint32
		kringMask    *uint32
		kringEntries *uint32
		kflags       *uint32
		kdropped     *uint32
		array        []uint32
		sqes         []SQE
		sqeHead      uint32
		sqeTail      uint32
		ringSz       uint32
		sqRingFd     unsafe.Pointer
		pad          [4]uint32
	}

	// IO completion data structure (Completion Queue Entry)
	CQE struct {
		UserData uint64
		Res      int32
		Flags    uint32
	}

	cQueue struct {
		khead        *uint32
		ktail        *uint32
		kringMask    *uint32
		kringEntries *uint32
		kflags       *uint32
		koverflow    *uint32
		cqes         []CQE
		ringSz       uint32
		cqRingFd     unsafe.Pointer
		pad          [4]uint32
	}

	// offsets for mmap
	ioSqOffsets struct {
		head        uint32
		tail        uint32
		ringMask    uint32
		ringEntries uint32
		flags       uint32
		dropped     uint32
		array       uint32
		resv1       uint32
		resv2       uint64
	}

	ioCqOffsets struct {
		head        uint32
		tail        uint32
		ringMask    uint32
		ringEntries uint32
		overflow    uint32
		cqes        uint32
		resv        [2]uint64
	}

	ioParams struct {
		sqEntries    uint32
		cqEntries    uint32
		flags        uint32
		sqThreadCPU  uint32
		sqThreadIdle uint32
		features     uint32
		resv         [4]uint32
		sqOff        ioSqOffsets
		cqOff        ioCqOffsets
	}
)
