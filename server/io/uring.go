package io

// #include <syscall.h>
//
// int get_setup_num(void) {
// #if defined(__NR_io_uring_setup)
//     return __NR_io_uring_setup;
// #else
//     return -1;
// #endif
// }
//
// int get_enter_num(void) {
// #if defined(__NR_io_uring_enter)
//     return __NR_io_uring_enter;
// #else
//     return -1;
// #endif
// }
// int get_register_num(void) {
// #if defined(__NR_io_uring_register)
// 		return __NR_io_uring_register;
// #else
// 		return -1;
// #endif
// }
import "C"
import (
	"fmt"
	"log"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type Ring struct {
	sq       sQueue
	cq       cQueue
	flags    uint32
	ringFd   int
	features uint32
}

const (
	ioringOffSqRing      = uint64(0x0)
	ioringOffCqRing      = uint64(0x8000000)
	ioringOffSqes        = uint64(0x10000000)
	ioringFeatSingleMmap = uint32(0x1)
	ioringEnterGetEvents = uint64(1) << 0
)

const (
	opRead  = 22
	opWrite = 23
)

const (
	defaultEntries = 256
)

var ioUringSetupSys, ioUringEnterSys, ioUringRegisterSys = func() (int, int, int) {
	ioSetupSys := C.get_setup_num()
	ioEnterSys := C.get_enter_num()
	ioRegisterSys := C.get_register_num()
	if ioSetupSys == -1 || ioEnterSys == -1 || ioRegisterSys == -1 {
		panic("io_uring is not supported")
	}

	return int(ioSetupSys), int(ioEnterSys), int(ioRegisterSys)
}()

func unmap(sq *sQueue, cq *cQueue) {
	_, _, _ = syscall.RawSyscall(syscall.SYS_MUNMAP, uintptr(sq.sqRingFd), uintptr(sq.ringSz), 0)
	if cq.cqRingFd != nil && cq.cqRingFd != sq.sqRingFd {
		_, _, _ = syscall.RawSyscall(syscall.SYS_MUNMAP, uintptr(cq.cqRingFd), uintptr(cq.ringSz), 0)
	}
}

func enter(ringFD int, toSubmit, minComplete, flags uint32) (int, syscall.Errno) {
	ret, _, err := syscall.RawSyscall6(uintptr(ioUringEnterSys), uintptr(ringFD), uintptr(toSubmit), uintptr(minComplete), uintptr(flags), 0, 0)
	return int(ret), err
}

func NewRingDefault() (*Ring, error) {
	var r Ring
	var p ioParams
	p.flags |= uint32(1<<8) | uint32(1<<7)

	r1, _, err := syscall.RawSyscall(uintptr(ioUringSetupSys), uintptr(defaultEntries), uintptr(unsafe.Pointer(&p)), 0)
	if err != 0 {
		return nil, fmt.Errorf("io_uring_setup: %v", err)
	}

	r.ringFd = int(r1)
	r.sq.ringSz = p.sqOff.array + p.sqEntries*uint32(unsafe.Sizeof(uint32(0)))
	r.cq.ringSz = p.cqOff.cqes + p.cqEntries*uint32(unsafe.Sizeof(CQE{}))

	sqPtr, _, err := syscall.RawSyscall6(
		syscall.SYS_MMAP, 0,
		uintptr(r.sq.ringSz),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(r.ringFd),
		uintptr(ioringOffSqRing))
	if err != 0 {
		return nil, fmt.Errorf("mmap on sqptr: %v", err)
	}
	r.sq.sqRingFd = unsafe.Pointer(sqPtr)

	if p.features&ioringFeatSingleMmap != 0 {
		r.cq.cqRingFd = r.sq.sqRingFd
	} else {
		cqPtr, _, e := syscall.RawSyscall6(
			syscall.SYS_MMAP,
			0,
			uintptr(r.cq.ringSz),
			syscall.PROT_READ|syscall.PROT_WRITE,
			syscall.MAP_SHARED|syscall.MAP_POPULATE,
			uintptr(r.ringFd),
			uintptr(ioringOffCqRing))
		if e != 0 {
			unmap(&r.sq, &r.cq)
			return nil, fmt.Errorf("mmap on cqptr: %v", err)
		}
		r.cq.cqRingFd = unsafe.Pointer(cqPtr)
	}

	sq := &r.sq
	sq.khead = (*uint32)(unsafe.Pointer(unsafe.Add(r.sq.sqRingFd, p.sqOff.head)))
	sq.ktail = (*uint32)(unsafe.Pointer(unsafe.Add(r.sq.sqRingFd, p.sqOff.tail)))
	sq.kringMask = (*uint32)(unsafe.Pointer(unsafe.Add(r.sq.sqRingFd, p.sqOff.ringMask)))
	sq.kringEntries = (*uint32)(unsafe.Pointer(unsafe.Add(r.sq.sqRingFd, p.sqOff.ringEntries)))
	sq.kflags = (*uint32)(unsafe.Pointer(unsafe.Add(r.sq.sqRingFd, p.sqOff.flags)))
	sq.kdropped = (*uint32)(unsafe.Pointer(unsafe.Add(r.sq.sqRingFd, p.sqOff.dropped)))

	arr := unsafe.Slice(
		(*uint32)(unsafe.Pointer(unsafe.Add(r.sq.sqRingFd, p.sqOff.array))),
		int(p.sqEntries))
	sq.array = arr

	sqes, _, e := syscall.RawSyscall6(
		syscall.SYS_MMAP,
		0,
		uintptr(p.sqEntries*uint32(unsafe.Sizeof(SQE{}))),
		syscall.PROT_READ|syscall.PROT_WRITE,
		syscall.MAP_SHARED|syscall.MAP_POPULATE,
		uintptr(r.ringFd),
		uintptr(ioringOffSqes))
	if e != 0 {
		unmap(&r.sq, &r.cq)
		return nil, fmt.Errorf("mmap on sqes array: %v", e.Error())
	}
	sqeSlice := unsafe.Slice((*SQE)(unsafe.Pointer(sqes)), int(p.sqEntries))
	sq.sqes = sqeSlice

	cq := &r.cq
	cq.khead = (*uint32)(unsafe.Pointer(unsafe.Add(r.cq.cqRingFd, p.cqOff.head)))
	cq.ktail = (*uint32)(unsafe.Pointer(unsafe.Add(r.cq.cqRingFd, p.cqOff.tail)))
	cq.kringMask = (*uint32)(unsafe.Pointer(unsafe.Add(r.cq.cqRingFd, p.cqOff.ringMask)))
	cq.kringEntries = (*uint32)(unsafe.Pointer(unsafe.Add(r.cq.cqRingFd, p.cqOff.ringEntries)))
	cq.koverflow = (*uint32)(unsafe.Pointer(unsafe.Add(r.cq.cqRingFd, p.cqOff.overflow)))

	cqeSlice := unsafe.Slice((*CQE)(unsafe.Pointer(unsafe.Add(r.cq.cqRingFd, p.cqOff.cqes))), int(p.cqEntries))
	cq.cqes = cqeSlice

	r.features = p.features

	return &r, nil
}

// SQ - Submissions Queue
func (r *Ring) submitToSQ(op uint8, fd int32, addr uintptr, len uint32, userData uint64) int {
	tail := atomic.LoadUint32(r.sq.ktail)
	index := tail & atomic.LoadUint32(r.sq.kringMask)

	sqe := &r.sq.sqes[index]
	sqe.opcode = op
	sqe.fd = fd
	sqe.addr = uint64(addr)
	sqe.len = len
	sqe.userData = userData

	r.sq.array[index] = index
	tail += 1

	atomic.StoreUint32(r.sq.ktail, tail)

	ret, _ := enter(r.ringFd, 1, 0, 0)
	return ret
}

// CQ - Completions Queue
func (r *Ring) readFromCQ() (CQE, bool) {
	head := atomic.LoadUint32(r.cq.khead)

	if head == atomic.LoadUint32(r.cq.ktail) {
		return CQE{}, false // empty buffer
	}

	defer atomic.StoreUint32(r.cq.khead, head+1)

	cqe := r.cq.cqes[head&atomic.LoadUint32(r.cq.kringMask)]
	return cqe, true
}

func (r *Ring) SubmitMultishot(pool *internalPool, sockfd int32) (int, syscall.Errno) {
	tail := atomic.LoadUint32(r.sq.ktail)
	index := tail & atomic.LoadUint32(r.sq.kringMask)

	sqe := &r.sq.sqes[index]
	*sqe = SQE{}
	sqe.opcode = IORING_OP_RECVMSG
	sqe.ioprio = IORING_RECV_MULTISHOT
	sqe.flags = IOSQE_BUFFER_SELECT
	sqe.fd = sockfd
	sqe.addr = uint64(uintptr(unsafe.Pointer(&unix.Msghdr{
		Namelen: uint32(unsafe.Sizeof(syscall.RawSockaddrAny{})),
	})))
	sqe.bufidx = pool.bgid
	sqe.userData = 1
	sqe.len = 1

	log.Printf("%#v\n", *sqe)

	r.sq.array[index] = index
	tail += 1

	atomic.StoreUint32(r.sq.ktail, tail)

	ret, err := enter(r.ringFd, 1, 1, 0)
	return ret, err
}

func (r *Ring) Fd() int {
	return r.ringFd
}

func (r *Ring) WaitForCQ() syscall.Errno {
	_, err := enter(r.ringFd, 0, 1, uint32(ioringEnterGetEvents))
	return err
}

func (r *Ring) ReadFromCQ() (CQE, bool) {
	return r.readFromCQ()
}
