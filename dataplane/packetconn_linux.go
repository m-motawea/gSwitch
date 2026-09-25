//go:build linux

package dataplane

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// vlanTagLen is the size of an 802.1Q tag (TPID + TCI).
const vlanTagLen = 4

// PacketConn is an AF_PACKET socket bound to one interface.
//
// Linux (and most NICs, including veth) strips the outermost VLAN tag on
// receive (rx-vlan-offload) and reports it out of band. A plain read of the
// socket therefore never sees the 802.1Q header. PacketConn enables
// PACKET_AUXDATA and re-inserts the tag into the frame so that trunk ports see
// exactly what was on the wire.
type PacketConn struct {
	ifi  *net.Interface
	file *os.File
	addr unix.SockaddrLinklayer
}

func htons(v uint16) uint16 {
	return v<<8 | v>>8
}

func ListenPacket(ifi *net.Interface) (*PacketConn, error) {
	proto := htons(unix.ETH_P_ALL)
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(proto))
	if err != nil {
		return nil, fmt.Errorf("socket: %w", err)
	}
	addr := unix.SockaddrLinklayer{Protocol: proto, Ifindex: ifi.Index}
	if err := unix.Bind(fd, &addr); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("bind %s: %w", ifi.Name, err)
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_PACKET, unix.PACKET_AUXDATA, 1); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("setsockopt PACKET_AUXDATA: %w", err)
	}
	// os.NewFile registers the non-blocking fd with the runtime poller, so
	// Close unblocks a pending read just like the previous raw.Conn did.
	return &PacketConn{
		ifi:  ifi,
		file: os.NewFile(uintptr(fd), "packet:"+ifi.Name),
		addr: addr,
	}, nil
}

// ReadFrame reads one frame into buf and returns the slice of buf holding it,
// with the VLAN tag restored if the kernel stripped it. buf must have room for
// the frame plus vlanTagLen bytes.
func (c *PacketConn) ReadFrame(buf []byte) ([]byte, error) {
	if len(buf) <= vlanTagLen {
		return nil, errors.New("buffer too small")
	}
	rc, err := c.file.SyscallConn()
	if err != nil {
		return nil, err
	}
	oob := make([]byte, unix.CmsgSpace(int(unsafe.Sizeof(unix.TpacketAuxdata{}))))
	var n, oobn int
	var recvErr error
	// Leave vlanTagLen bytes at the front so the tag can be inserted by
	// shifting only the 12 address bytes.
	err = rc.Read(func(fd uintptr) bool {
		n, oobn, _, _, recvErr = unix.Recvmsg(int(fd), buf[vlanTagLen:], oob, 0)
		return recvErr != unix.EAGAIN
	})
	if err != nil {
		return nil, err
	}
	if recvErr != nil {
		return nil, recvErr
	}
	frame := buf[vlanTagLen : vlanTagLen+n]

	aux, ok := parseAuxdata(oob[:oobn])
	// Kernels before TP_STATUS_VLAN_VALID only reported a non-zero TCI.
	if !ok || (aux.Status&unix.TP_STATUS_VLAN_VALID == 0 && aux.Vlan_tci == 0) || n < 12 {
		return frame, nil
	}
	tpid := uint16(unix.ETH_P_8021Q)
	if aux.Status&unix.TP_STATUS_VLAN_TPID_VALID != 0 {
		tpid = aux.Vlan_tpid
	}
	copy(buf[0:12], frame[0:12])
	binary.BigEndian.PutUint16(buf[12:14], tpid)
	binary.BigEndian.PutUint16(buf[14:16], aux.Vlan_tci)
	return buf[:n+vlanTagLen], nil
}

func parseAuxdata(oob []byte) (unix.TpacketAuxdata, bool) {
	msgs, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return unix.TpacketAuxdata{}, false
	}
	for _, m := range msgs {
		if m.Header.Level != unix.SOL_PACKET || m.Header.Type != unix.PACKET_AUXDATA {
			continue
		}
		if len(m.Data) < int(unsafe.Sizeof(unix.TpacketAuxdata{})) {
			return unix.TpacketAuxdata{}, false
		}
		return *(*unix.TpacketAuxdata)(unsafe.Pointer(&m.Data[0])), true
	}
	return unix.TpacketAuxdata{}, false
}

// WriteFrame sends a complete Ethernet frame (including any in-band VLAN tag)
// out of the bound interface.
func (c *PacketConn) WriteFrame(frame []byte) (int, error) {
	rc, err := c.file.SyscallConn()
	if err != nil {
		return 0, err
	}
	var sendErr error
	err = rc.Write(func(fd uintptr) bool {
		sendErr = unix.Sendto(int(fd), frame, 0, &c.addr)
		return sendErr != unix.EAGAIN
	})
	if err != nil {
		return 0, err
	}
	if sendErr != nil {
		return 0, sendErr
	}
	return len(frame), nil
}

func (c *PacketConn) Close() error {
	return c.file.Close()
}
