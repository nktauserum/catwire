// Package tun provides a TUN interface implementation
package tun

import (
	"os"

	"golang.org/x/sys/unix"
)

type TUN struct {
	file *os.File
	name string
}

func CreateTUN(name string) (*TUN, error) {
	nfd, err := unix.Open("/dev/net/tun", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		return nil, err
	}

	err = unix.IoctlIfreq(nfd, unix.TUNSETIFF, ifr)
	if err != nil {
		return nil, err
	}

	err = unix.SetNonblock(nfd, true)
	if err != nil {
		unix.Close(nfd)
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

