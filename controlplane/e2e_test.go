package controlplane_test

import (
	"encoding/binary"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/dataplane"
	_ "github.com/m-motawea/gSwitch/l2"
	_ "github.com/m-motawea/gSwitch/l3"
	"github.com/m-motawea/icmp"
	"github.com/m-motawea/ip"
	"github.com/mdlayher/ethernet"
)

func setupE2ESwitch() (*controlplane.Switch, *sync.WaitGroup) {
	cfg := config.Config{
		ControlProcess: []config.ControlProcessConfig{
			{Layer: 2, Name: "L2Switch"},
			{Layer: 2, Name: "ARP", ConfigFile: "../testdata/arp.toml"},
			{Layer: 2, Name: "L2Adapter", ConfigFile: "../testdata/l2adapter.toml"},
			{Layer: 3, Name: "IPv4"},
			{Layer: 3, Name: "Routing", ConfigFile: "../testdata/routing.toml"},
			{Layer: 3, Name: "ICMP", ConfigFile: "../testdata/icmp.toml"},
		},
	}
	var wg sync.WaitGroup
	// NewSwitch initializes and injects the pipeline properly
	sw := controlplane.NewSwitch("e2e_switch", cfg, &wg)

	sw.Ports["eth0"] = &dataplane.SwitchPort{
		Name:      "eth0",
		OutBuf:    make(chan *ethernet.Frame, 10),
		Status:    true,
	}
	// sw.dataPlaneChan is globally hidden but we injected a Testing Expose method!
	sw.Start()
	time.Sleep(200 * time.Millisecond)
	return sw, &wg
}

func TestSwitchEndToEnd_ICMP(t *testing.T) {
	// Tests L2(ARP/Mock) -> L3(IPv4 Decoder) -> ICMP Process -> (L3/IPv4 Encoder) -> OutPort natively!
	sw, _ := setupE2ESwitch()
	defer func() {
		go sw.Stop()
		time.Sleep(100 * time.Millisecond)
	}()

	ic := icmp.ICMP{
		Type: icmp.TYPE_ICMP_ECHO_REPQUEST,
		Code: 0,
		Data: []byte("e2eping"),
	}
	icData, _ := ic.MarshalBinary()

	ipHeader := ip.IPv4{
		Version:     4,
		HLEN:        5,
		TypeOfService: 0,
		TotalLength: ip.TotalLength(20 + len(icData)),
		TTL:         64,
		Source:      ip.IP(binary.BigEndian.Uint32(net.ParseIP("10.0.0.50").To4())),
		Destination: ip.IP(binary.BigEndian.Uint32(net.ParseIP("10.0.0.1").To4())),
		Protocol:    ip.PROTO_ICMP,
		Data:        icData,
	}
	ipData, _ := ipHeader.MarshalBinary()

	frame := &ethernet.Frame{
		Destination: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
		Source:      net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66},
		EtherType:   ethernet.EtherTypeIPv4,
		Payload:     ipData,
	}

	inFrame := dataplane.IncomingFrame{
		FRAME:   frame,
		IN_PORT: sw.Ports["eth0"],
	}

	// Emit natively
	log.Println("E2E TESTS: Injecting Packet to dataPlaneChan...")
	sw.InjectTestFrame(inFrame)
	log.Println("E2E TESTS: Injection successful, waiting for ARP Request...")

	// 1. First we expect an ARP Request because the switch doesn't know 10.0.0.50's MAC natively
	select {
	case outFrame := <-sw.Ports["eth0"].OutBuf:
		if outFrame.EtherType != ethernet.EtherTypeARP {
			t.Fatalf("Expected ARP Request, got %v", outFrame.EtherType)
		}
		log.Println("E2E TESTS: Received ARP Request successfully. Generating ARP Reply...")
		// Generate ARP Reply
		arpReplyPayload := []byte{
			0x00, 0x01, 0x08, 0x00, 0x06, 0x04, 0x00, 0x02, // Hardware Ethernet, Protocol IPv4, HW Len 6, Proto Len 4, Opcode 2 (Reply)
			0x11, 0x22, 0x33, 0x44, 0x55, 0x66,             // Sender MAC (10.0.0.50)
			10, 0, 0, 50,                                   // Sender IP (10.0.0.50)
			0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff,             // Target MAC (10.0.0.1)
			10, 0, 0, 1,                                    // Target IP (10.0.0.1)
		}
		replyFrame := &ethernet.Frame{
			Destination: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			Source:      net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66},
			EtherType:   ethernet.EtherTypeARP,
			Payload:     arpReplyPayload,
		}
		replyInFrame := dataplane.IncomingFrame{
			FRAME:   replyFrame,
			IN_PORT: sw.Ports["eth0"],
		}
		sw.InjectTestFrame(replyInFrame)

	case <-time.After(8 * time.Second):
		t.Fatalf("Timeout waiting for ARP Request on E2E test")
	}

	log.Println("E2E TESTS: ARP Reply Injected. Waiting for ICMP IPv4 Response...")

	// 2. Second we expect the queued ICMP IPv4 packet to flush!
	select {
	case outFrame := <-sw.Ports["eth0"].OutBuf:
		if outFrame.EtherType != ethernet.EtherTypeIPv4 {
			t.Fatalf("Expected IPv4 Ethernet response, got %v", outFrame.EtherType)
		}
		var resultIP ip.IPv4
		if err := (&resultIP).UnmarshalBinary(outFrame.Payload); err != nil {
			t.Fatalf("Failed unpacking response IPv4 Frame")
		}
		var resultICMP icmp.ICMP
		if err := (&resultICMP).UnmarshalBinary(resultIP.Data); err != nil {
			t.Fatalf("Failed unpacking response ICMP segment")
		}
		if resultICMP.Type != icmp.TYPE_ICMP_ECHO_REPLY {
			t.Fatalf("E2E failed. ICMP Type is %d, expected %d", resultICMP.Type, icmp.TYPE_ICMP_ECHO_REPLY)
		}
		log.Println("E2E TESTS: ICMP Echo Reply validated successfully!")
	case <-time.After(8 * time.Second):
		t.Fatalf("Timeout waiting for ICMP Pipeline Reply on E2E test")
	}
}
