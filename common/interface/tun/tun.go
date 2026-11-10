// Package tun provides a TUN interface implementation
package tun

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	cIFFTUN        = 0x0001
	cIFFTAP        = 0x0002
	cIFFNOPI       = 0x1000
	cIFFMULTIQUEUE = 0x0100
)

type TUN struct {
	file *os.File
	name string
}

func CreateTUN(name string) (*TUN, error) {
	nfd, err := syscall.Open("/dev/net/tun", unix.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		return nil, err
	}

	var flags uint16 = cIFFNOPI | cIFFTUN
	ifr.SetUint16(flags)

	err = unix.IoctlIfreq(nfd, unix.TUNSETIFF, ifr)
	if err != nil {
		return nil, err
	}

	fd := os.NewFile(uintptr(nfd), "/dev/net/tun")

	return &TUN{
		file: fd,
	}, nil
}

func (tun *TUN) Write(data []byte) (int, error) {
	return tun.file.Write(data)
}

func (tun *TUN) Read(buf []byte) (int, error) {
	return tun.file.Read(buf)
}
