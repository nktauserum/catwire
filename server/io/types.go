package io

import (
	"unsafe"
)

type (
	SQE struct {
		opcode      uint8  /* type of operation for this sqe */
		flags       uint8  /* IOSQE_ flags */
		ioprio      uint16 /* ioprio for the request */
		fd          int32  /* file descriptor to do IO on */
		off         uint64 /* offset into file */
		addr        uint64 /* pointer to buffer or iovecs */
		len         uint32 /* buffer size or number of iovecs */
		sqeFlags    uint32
		userData    uint64 /* data to be passed back at completion time */
		bufidx      uint16
		personality uint16 /* personality to use, if used */
		spliceIn    uint32 /* splice_fd_in / file_index / optlen union */
		_           [16]byte
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

const (
	IORING_OP_NOP = iota
	IORING_OP_READV
	IORING_OP_WRITEV
	IORING_OP_FSYNC
	IORING_OP_READ_FIXED
	IORING_OP_WRITE_FIXED
	IORING_OP_POLL_ADD
	IORING_OP_POLL_REMOVE
	IORING_OP_SYNC_FILE_RANGE
	IORING_OP_SENDMSG
	IORING_OP_RECVMSG
	IORING_OP_TIMEOUT
	IORING_OP_TIMEOUT_REMOVE
	IORING_OP_ACCEPT
	IORING_OP_ASYNC_CANCEL
	IORING_OP_LINK_TIMEOUT
	IORING_OP_CONNECT
	IORING_OP_FALLOCATE
	IORING_OP_OPENAT
	IORING_OP_CLOSE
	IORING_OP_FILES_UPDATE
	IORING_OP_STATX
	IORING_OP_READ
	IORING_OP_WRITE
	IORING_OP_FADVISE
	IORING_OP_MADVISE
	IORING_OP_SEND
	IORING_OP_RECV
	IORING_OP_OPENAT2
	IORING_OP_EPOLL_CTL
	IORING_OP_SPLICE
	IORING_OP_PROVIDE_BUFFERS
	IORING_OP_REMOVE_BUFFERS
	IORING_OP_TEE
	IORING_OP_SHUTDOWN
	IORING_OP_RENAMEAT
	IORING_OP_UNLINKAT
	IORING_OP_MKDIRAT
	IORING_OP_SYMLINKAT
	IORING_OP_LINKAT
	IORING_OP_MSG_RING
	IORING_OP_FSETXATTR
	IORING_OP_SETXATTR
	IORING_OP_FGETXATTR
	IORING_OP_GETXATTR
	IORING_OP_SOCKET
	IORING_OP_URING_CMD
	IORING_OP_SEND_ZC
	IORING_OP_SENDMSG_ZC
	IORING_OP_LAST
)
