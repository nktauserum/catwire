package session

import "net"

type IncomingCallback func([]byte)
type OutgoingCallback func([]byte, *net.UDPAddr)
