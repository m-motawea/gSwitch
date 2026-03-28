package l3

import (
	"encoding/binary"
	"log"
	"net"
	"sync"
	"testing"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/icmp"
	"github.com/m-motawea/ip"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/ethernet"
)

func setupICMPSwitch() *controlplane.Switch {
	cfg := config.Config{}
	var wg sync.WaitGroup
	sw := controlplane.NewSwitch("test_switch", cfg, &wg)

	InitICMP(sw)

	// Inject custom config manually since we skip reading from file in unit tests
	stor := sw.Stor.GetStor(3, "ICMP")
	stor["CONFIG"] = ICMPConfig{
		LocalAddresses: map[string]LocalAddress{
			"eth0": {Address: "10.0.0.1"},
		},
	}

	return sw
}

func createICMPMsg(sw *controlplane.Switch, icmpType icmp.Type, srcIP string, dstIP string) pipeline.PipelineMessage {
	// Create raw ICMP packet
	ic := icmp.ICMP{
		Type: icmpType,
		Code: 0,
		Data: []byte("ping_data"),
	}
	icData, err := ic.MarshalBinary()
	if err != nil {
		log.Fatalf("Mock ICMP error: %v", err)
	}

	// Wrap inside IPv4 Payload mimicking L3Adapter Output
	ipPayload := ip.IPv4{
		Source:      ip.IP(binary.BigEndian.Uint32(net.ParseIP(srcIP).To4())),
		Destination: ip.IP(binary.BigEndian.Uint32(net.ParseIP(dstIP).To4())),
		Protocol:    ip.PROTO_ICMP,
		Data:        icData,
	}

	inFrame := dataplane.IncomingFrame{
		FRAME:   &ethernet.Frame{}, // Dummy wrapper
		IN_PORT: &dataplane.SwitchPort{Name: "dummy_port"},
	}

	ctrlMsg := controlplane.ControlMessage{
		InFrame:      &inFrame,
		ParentSwitch: sw,
		LayerPayload: ipPayload, // Inject our prepared structure natively
	}

	return pipeline.PipelineMessage{
		Direction: pipeline.PipelineInDirection,
		Content:   controlplane.StoreMessage(ctrlMsg),
	}
}

func TestICMPProcessIn_GeneratesEchoReplyLocally(t *testing.T) {
	sw := setupICMPSwitch()

	// Send an Echo Request targeting the internal IP "10.0.0.1"
	pipeMsg := createICMPMsg(sw, icmp.TYPE_ICMP_ECHO_REPQUEST, "10.0.0.50", "10.0.0.1")

	resultMsg := ICMPProcessIn(pipeline.PipelineProcess{}, pipeMsg)

	if !resultMsg.Finished {
		t.Errorf("Expected ICMP to swallow Echo Request locally and set Finished = true")
	}

	ctrlMsg, ok := controlplane.FetchMessage(resultMsg.Content)
	if !ok {
		t.Fatalf("Failed to decode mapped Switch pointer")
	}

	modifiedIP, ok := ctrlMsg.LayerPayload.(ip.IPv4)
	if !ok {
		t.Fatalf("Failed to cast resulting LayerPayload to IPv4")
	}

	// Verify IPs swapped properly
	srcStr := make(net.IP, 4)
	binary.BigEndian.PutUint32(srcStr, uint32(modifiedIP.Source))
	if srcStr.String() != "10.0.0.1" {
		t.Errorf("Expected Source IP 10.0.0.1, got %v", srcStr.String())
	}
	
	dstStr := make(net.IP, 4)
	binary.BigEndian.PutUint32(dstStr, uint32(modifiedIP.Destination))
	if dstStr.String() != "10.0.0.50" {
		t.Errorf("Expected Destination IP 10.0.0.50, got %v", dstStr.String())
	}

	// Verify ICMP Type is now Echo Reply (Type 0)
	var modifiedICMP icmp.ICMP
	if err := modifiedICMP.UnmarshalBinary(modifiedIP.Data); err != nil {
		t.Fatalf("Failed to unmarshal returned ICMP payload")
	}

	if modifiedICMP.Type != icmp.TYPE_ICMP_ECHO_REPLY {
		t.Errorf("Expected ICMP Type 0 (Echo Reply), got %v", modifiedICMP.Type)
	}
}

func TestICMPProcessIn_IgnoresForeignEchoRequest(t *testing.T) {
	sw := setupICMPSwitch()

	// Send an Echo Request targeting a FOREIGN IP "10.0.0.254"
	pipeMsg := createICMPMsg(sw, icmp.TYPE_ICMP_ECHO_REPQUEST, "10.0.0.50", "10.0.0.254")

	resultMsg := ICMPProcessIn(pipeline.PipelineProcess{}, pipeMsg)

	// Switch should completely ignore it, allowing pipeline to advance into routing phase
	if resultMsg.Finished {
		t.Errorf("Expected non-local ICMP requests to bypass consumption (Finished=false)")
	}
	if resultMsg.Drop {
		t.Errorf("Expected non-local ICMP requests to NOT drop natively")
	}
}
