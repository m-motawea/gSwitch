//go:build !linux

package dataplane

import (
	"errors"
	"net"
)

// vlanTagLen is the size of an 802.1Q tag (TPID + TCI).
const vlanTagLen = 4

var errUnsupported = errors.New("dataplane: AF_PACKET sockets are only supported on linux")

// PacketConn is only implemented on linux.
type PacketConn struct{}

func ListenPacket(ifi *net.Interface) (*PacketConn, error) {
	return nil, errUnsupported
}

func (c *PacketConn) ReadFrame(buf []byte) ([]byte, error) {
	return nil, errUnsupported
}

func (c *PacketConn) WriteFrame(frame []byte) (int, error) {
	return 0, errUnsupported
}

func (c *PacketConn) Close() error {
	return errUnsupported
}
