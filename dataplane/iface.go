package dataplane

import (
	"log"
	"net"
	"time"

	"github.com/mdlayher/raw"
)

const ETH_P_ALL = 0x0003
const IFACE_BUFFER_SIZE = 10

type Iface interface {
	SendLoop(chan int)
	RecvLoop(chan IncomingFrame, chan int)
	Out([]byte) error
}

type SwitchPort struct {
	Name         string
	IFI          *net.Interface
	Conn         *raw.Conn
	VLAN         int
	Status       bool
	OutBuf       chan []byte
	Trunk        bool
	AllowedVLANs []int
	closeSend    chan int
	closeRecv    chan int
}

type IncomingFrame struct {
	FRAME    []byte
	SRC_ADDR net.Addr
	IN_PORT  *SwitchPort
}

func (s *SwitchPort)addVlanTag(frame []byte) []byte{
	return frame
}

func (s *SwitchPort) SendLoop(close chan int) {
	log.Printf("SWPORT: Staring SendLoop for Port: %s", s.Name)
	defer log.Printf("Port: %s SendLoop Stopping..", s.Name)
	for {
		select {
		case <-close:
			return
		default:
			frame := <-s.OutBuf
			outFrame := s.addVlanTag(frame)
			n, err := s.Conn.WriteTo(outFrame, s.Conn.LocalAddr())
			if err != nil {
				log.Printf("Failed to send frame out of interface %s due toi error: %t", s.Name, err)
			}
			log.Printf("%d bytes sent out of port %s", n, s.Name)
		}
	}
}

func (s *SwitchPort) RecvLoop(controlChannel chan IncomingFrame, close chan int) {
	log.Printf("SWPORT: Staring RecvLoop for Port: %s", s.Name)
	defer log.Printf("Port: %s RecvLoop Stopping..", s.Name)
	for {
		select {
		case <-close:
			return
		default:
			buf := make([]byte, s.IFI.MTU)
			n, addr, err := s.Conn.ReadFrom(buf)
			if err != nil {
				log.Printf("Failed to receive on interface %s due to error: %t", s.Name, err)
			} else {
				log.Printf("%d bytes received on port %s", n, s.Name)
				f_pair := IncomingFrame{
					FRAME:    buf[:n],
					SRC_ADDR: addr,
					IN_PORT:  s,
				}
				controlChannel <- f_pair
			}
		}
	}
}

func (s *SwitchPort) Out(frame []byte) {
	s.OutBuf <- frame
}

func NewSwitchPort(ifname string, isTrunk bool, vlans ...int) (SwitchPort, error) {
	sendCloseChannel := make(chan int)
	recvCloseChannel := make(chan int)
	iface := SwitchPort{}
	iface.Name = ifname
	iface.closeSend = sendCloseChannel
	iface.closeRecv = recvCloseChannel
	ifi, err := net.InterfaceByName(ifname)
	if err != nil {
		log.Printf("Failed to get port %s due to error: %t\n", ifname, err)
		return iface, err
	}
	iface.IFI = ifi
	iface.Trunk = isTrunk
	if iface.Trunk {
		iface.AllowedVLANs = vlans
	} else {
		iface.VLAN = vlans[0]
	}
	iface.OutBuf = make(chan []byte, IFACE_BUFFER_SIZE)
	return iface, nil
}

func (s *SwitchPort) Up(controlChannel chan IncomingFrame) error {
	c, err := raw.ListenPacket(s.IFI, ETH_P_ALL, nil)
	if err != nil {
		log.Printf("Failed to listen on port %s due to error %t", s.Name, err)
		return err
	}
	s.Conn = c
	time.Sleep(2 * time.Second)
	go s.SendLoop(s.closeSend)
	time.Sleep(2 * time.Second)
	go s.RecvLoop(controlChannel, s.closeRecv)
	s.Status = true
	return nil
}

func (s *SwitchPort) Down() error {
	s.Status = false
	s.closeRecv <- 1
	s.closeSend <- 1
	return s.Conn.Close()
}
