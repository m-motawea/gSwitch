package l2

import (
	"log"
	"net"
	"net/netip"
	"sync"
	"testing"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

// setupMockSwitch initializes a blank switch with an established ARP instance
func setupMockSwitch() *controlplane.Switch {
	cfg := config.Config{}
	var wg sync.WaitGroup
	sw := controlplane.NewSwitch("test_switch", cfg, &wg)

	// Initialize ARP
	InitARP(sw)

	stor := sw.Stor.GetStor(2, "ARP")
	stor["CONFIG"] = ARPConfig{
		LocalAddresses: map[string]LocalAddress{
			"eth0": {IP: "192.168.1.1", MAC: "aa:bb:cc:dd:ee:ff"},
		},
	}

	return sw
}

// Generate mock ARP pipeline message
func createArpPipelineMsg(sw *controlplane.Switch, op arp.Operation, senderMac, targetMac net.HardwareAddr, senderIp, targetIp net.IP) pipeline.PipelineMessage {
	srcAddr, _ := netip.AddrFromSlice(senderIp.To4())
	dstAddr, _ := netip.AddrFromSlice(targetIp.To4())

	p, err := arp.NewPacket(op, senderMac, srcAddr, targetMac, dstAddr)
	if err != nil {
		log.Fatal(err)
	}

	pb, err := p.MarshalBinary()
	if err != nil {
		log.Fatal(err)
	}

	frame := &ethernet.Frame{
		Destination: targetMac,
		Source:      senderMac,
		EtherType:   ethernet.EtherTypeARP,
		Payload:     pb,
	}

	inFrame := dataplane.IncomingFrame{
		FRAME:   frame,
		IN_PORT: &dataplane.SwitchPort{Name: "dummy_port"},
	}

	ctrlMsg := controlplane.ControlMessage{
		InFrame:      &inFrame,
		ParentSwitch: sw,
		OutPorts:     []*dataplane.SwitchPort{},
	}

	pipeMsg := pipeline.PipelineMessage{
		Direction: pipeline.PipelineInDirection,
		Content:   controlplane.StoreMessage(ctrlMsg),
	}

	return pipeMsg
}

func TestReplyARPIn_GeneratesReplyToLocalIP(t *testing.T) {
	sw := setupMockSwitch()

	senderMac, _ := net.ParseMAC("11:22:33:44:55:66")
	targetMac, _ := net.ParseMAC("ff:ff:ff:ff:ff:ff") // Broadcast request

	// Send ARP Request resolving "192.168.1.1" which matches the Switch's config!
	pipeMsg := createArpPipelineMsg(
		sw,
		arp.OperationRequest,
		senderMac,
		targetMac,
		net.ParseIP("192.168.1.100"),
		net.ParseIP("192.168.1.1"),
	)

	// Execute Process
	resultMsg := ReplyARPIn(pipeline.PipelineProcess{}, pipeMsg)

	// Since we mapped it using StoreMessage, we must fetch it natively
	ctrlMsg, ok := controlplane.FetchMessage(resultMsg.Content)
	if !ok {
		t.Fatalf("Failed to fetch resolved control message")
	}

	if !resultMsg.Finished {
		t.Errorf("Expected msg.Finished = true because switch should natively reply to internal IP")
	}

	replyFrame := ctrlMsg.InFrame.FRAME
	if replyFrame.Source.String() != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("Expected source MAC aa:bb:cc:dd:ee:ff, got %v", replyFrame.Source.String())
	}
	if replyFrame.Destination.String() != senderMac.String() {
		t.Errorf("Expected destination MAC %v, got %v", senderMac.String(), replyFrame.Destination.String())
	}

	// Unmarshal payload to ensure it is actually a Reply
	rp := new(arp.Packet)
	if err := rp.UnmarshalBinary(replyFrame.Payload); err != nil {
		t.Fatalf("Failed to unmarshal returned ARP payload")
	}

	if rp.Operation != arp.OperationReply {
		t.Errorf("Expected operation to be Reply, got %v", rp.Operation)
	}
}

func TestReplyARPIn_PopulateTableOnForeignRequest(t *testing.T) {
	sw := setupMockSwitch()

	senderMac, _ := net.ParseMAC("11:22:33:44:55:66")
	targetMac, _ := net.ParseMAC("ff:ff:ff:ff:ff:ff") // Broadcast request

	// Send ARP Request resolving "192.168.1.50" which is FORIEGN
	pipeMsg := createArpPipelineMsg(
		sw,
		arp.OperationRequest,
		senderMac,
		targetMac,
		net.ParseIP("192.168.1.100"),
		net.ParseIP("192.168.1.50"),
	)

	// Execute Process
	resultMsg := ReplyARPIn(pipeline.PipelineProcess{}, pipeMsg)

	if !resultMsg.Finished {
		t.Errorf("Expected msg.Finished = true because switch halts pipeline for unanswered ARPs to emit instantly")
	}

	// Verify ARP Table has learned 192.168.1.100 -> 11:22:33:44:55:66
	stor := sw.Stor.GetStor(2, "ARP")
	table := stor["Table"].(SwitchARPTable)

	// Need to check the Entry!
	// Wait! The table is populated asynchronously sometimes or synchronously. SetEntry is synchronous.
	ent := table.GetEntry(net.ParseIP("192.168.1.100"))
	if ent == nil {
		t.Fatalf("Expected ARP entry for 192.168.1.100 to be recorded, but found nil")
	}

	if ent.MAC.String() != senderMac.String() {
		t.Errorf("Expected learned MAC to be %v, got %v", senderMac.String(), ent.MAC.String())
	}
}
