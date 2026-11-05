// Package tun provides a TUN interface implementation
package tun

import "os"

type TUN struct {
	file *os.File
}

