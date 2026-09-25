package dataplane

import (
	"log"
	"net"
	"time"

	"github.com/mdlayher/ethernet"
)

const IFACE_BUFFER_SIZE = 50
const TYPE_802_1Q = 0x8100

type Iface interface {
	SendLoop(chan int)
	RecvLoop(chan IncomingFrame, chan int)
	Out([]byte) error
}

type SwitchPort struct {
	Name         string
	IFI          *net.Interface
	Conn         *PacketConn
	VLAN         int
	Status       bool
	OutBuf       chan *ethernet.Frame
	Trunk        bool
	AllowedVLANs []int
	closeSend    chan int
	closeRecv    chan int
}

type IncomingFrame struct {
	FRAME   *ethernet.Frame
	IN_PORT *SwitchPort
}

func (s *SwitchPort) setSendVlanTag(frame *ethernet.Frame) []byte {
	// The same *ethernet.Frame is handed to every output port (e.g. when
	// flooding), and each port's SendLoop runs concurrently. Work on a copy so
	// an access port stripping the tag can't make a trunk port re-tag the
	// frame with the native VLAN.
	f := &ethernet.Frame{}
	*f = *frame
	if s.Trunk {
		log.Printf("sending out of trunk port %s", s.Name)
		// In case of Trunk Port
		if f.VLAN == nil {
			// if no vlan tag added it will add the Native VLAN tag
			log.Printf("trunk port %s sending: no vlan tag assigned. assigning native vlan %d", s.Name, s.AllowedVLANs[0])
			vlan := ethernet.VLAN{ID: uint16(s.AllowedVLANs[0])}
			f.VLAN = &vlan
			b, err := f.MarshalBinary()
			if err != nil {
				log.Printf("failed to marshal frame after adding VLAN in trunk")
				return []byte{}
			}
			return b
		} else {
			// If there is VLAN Tag specified it will check whether it is allowed on this port or not
			log.Printf("trunk port %s sending: vlan tag found. id: %d", s.Name, f.VLAN.ID)
			var FOUND bool
			for _, id := range s.AllowedVLANs {
				if f.VLAN == nil {
					return []byte{}
				}
				if id == int(f.VLAN.ID) {
					FOUND = true
					break
				}
			}
			if FOUND {
				b, err := f.MarshalBinary()
				if err != nil {
					log.Printf("failed to marshal frame after adding VLAN in trunk")
					return []byte{}
				}
				return b
			} else {
				log.Printf("trunk port %s sending: vlan id: %d not allowed on port %s", s.Name, f.VLAN.ID, s.Name)
				return []byte{}
			}
		}
	} else {
		// In case of Access Port
		if f.VLAN != nil {
			if int(f.VLAN.ID) != s.VLAN {
				// Discard
				// log.Printf("access port %s sending: vlan tag found. id: %d. Discarding", s.Name, f.VLAN.ID)
				return []byte{}
			}
			// Strip VLAN Tag
			log.Printf("access port %s sending: Stripping VLAN Tag.", s.Name)
			f.VLAN = nil
			b, err := f.MarshalBinary()
			if err != nil {
				log.Printf("failed to marshal frame after adding VLAN in trunk")
				return []byte{}
			}
			return b
		}
		b, err := f.MarshalBinary()
		if err != nil {
			log.Printf("failed to marshal frame after adding VLAN in trunk")
			return []byte{}
		}
		return b
	}
	b, err := f.MarshalBinary()
	if err != nil {
		log.Printf("failed to marshal frame after adding VLAN in trunk")
		return []byte{}
	}
	return b
}

func (s *SwitchPort) setRecvVlanTag(frame []byte) *ethernet.Frame {
	log.Printf("receiving on port %s, %v", s.Name, frame)
	var f ethernet.Frame
	if err := (&f).UnmarshalBinary(frame); err != nil {
		log.Printf("failed to unmarshal ethernet frame: %v", err)
		return nil
	}

	if s.Trunk {
		// In case of Trunk Port
		log.Printf("receiving on trunk port %s", s.Name)
		if f.VLAN == nil {
			// if no vlan tag added it will add the Native VLAN tag
			if len(s.AllowedVLANs) == 0 {
				return nil
			}
			log.Printf("trunk port %s receiving: no vlan tag assigned. assigning native vlan %d", s.Name, s.AllowedVLANs[0])
			vlan := ethernet.VLAN{ID: uint16(s.AllowedVLANs[0])}
			f.VLAN = &vlan
			return &f
		} else {
			// If there is VLAN Tag specified it will check whether it is allowed on this port or not
			log.Printf("trunk port %s receiving: vlan tag found. id: %d", s.Name, f.VLAN.ID)
			var FOUND bool
			for _, id := range s.AllowedVLANs {
				if id == int(f.VLAN.ID) {
					FOUND = true
					break
				}
			}
			if FOUND {
				return &f
			} else {
				log.Printf("trunk port %s receiving: vlan id: %d not allowed on port %s", s.Name, f.VLAN.ID, s.Name)
				return nil
			}
		}
	} else {
		// In case of Access Port
		if f.VLAN != nil {
			// Discard
			log.Printf("access port %s receiving: vlan tag found. id: %d. Discarding", s.Name, f.VLAN.ID)
			return nil
		}
		// Set VLAN Tag of the Port
		vlan := ethernet.VLAN{ID: uint16(s.VLAN)}
		f.VLAN = &vlan
		return &f
	}
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
			outFrame := s.setSendVlanTag(frame)
			log.Printf("sending out of port %s, %v", s.Name, outFrame)
			if len(outFrame) == 0 {
				continue
			}
			n, err := s.Conn.WriteFrame(outFrame)
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
			// MTU excludes the Ethernet header; leave room for the header,
			// a stacked (QinQ) tag and the tag ReadFrame re-inserts.
			buf := make([]byte, s.IFI.MTU+14+2*vlanTagLen)
			data, err := s.Conn.ReadFrame(buf)
			if err != nil {
				log.Printf("Failed to receive on interface %s due to error: %v", s.Name, err)
			} else {
				log.Printf("%d bytes received on port %s", len(data), s.Name)
				frame := s.setRecvVlanTag(data)
				if frame == nil {
					continue
				}
				log.Printf("frame with VLAN %v", frame.VLAN)
				f_pair := IncomingFrame{
					FRAME:   frame,
					IN_PORT: s,
				}
				controlChannel <- f_pair
			}
		}
	}
}

func (s *SwitchPort) Out(frame *ethernet.Frame) {
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
		iface.AllowedVLANs = vlans
	}
	iface.OutBuf = make(chan *ethernet.Frame, IFACE_BUFFER_SIZE)
	return iface, nil
}

func (s *SwitchPort) Up(controlChannel chan IncomingFrame) error {
	c, err := ListenPacket(s.IFI)
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
