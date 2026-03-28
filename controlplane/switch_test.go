package controlplane

import (
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/ethernet"
)

// InjectTestFrame exposes the private ingestion channel for e2e_test.go namespace assertions.
func (sw *Switch) InjectTestFrame(frame dataplane.IncomingFrame) {
	sw.dataPlaneChan <- frame
}

// Mock a L2 process directly to test the controlplane switch graph independently
func mockHubInProc(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, ok := FetchMessage(msg.Content)
	if !ok {
		log.Println("MockHub: failed downcast")
		return msg
	}
	// "Flood" to all other ports
	for _, port := range msgContent.ParentSwitch.Ports {
		if port.Name != msgContent.InFrame.IN_PORT.Name {
			msgContent.OutPorts = append(msgContent.OutPorts, port)
		}
	}
	msg.Content = StoreMessage(msgContent)
	return msg
}

func mockHubOutProc(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msg.Finished = true // Stop execution and output it
	return msg
}

func init() {
	ControlProcs = map[int]map[string]ControlProcessFuncPair{}
	var pair ControlProcessFuncPair
	pair.InFunc = mockHubInProc
	pair.OutFunc = mockHubOutProc
	RegisterLayerProc(2, "TestHub", pair)
}

func TestSwitchDataPlaneMock(t *testing.T) {
	// Configure test switch using our Mock Hub
	cfg := config.Config{
		ControlProcess: []config.ControlProcessConfig{
			{Layer: 2, Name: "TestHub"},
		},
	}
	var wg sync.WaitGroup
	sw := NewSwitch("test_switch", cfg, &wg)

	// Inject bypass ports
	port1 := &dataplane.SwitchPort{
		Name:   "port1",
		OutBuf: make(chan *ethernet.Frame, 10),
		Status: true,
	}
	port2 := &dataplane.SwitchPort{
		Name:   "port2",
		OutBuf: make(chan *ethernet.Frame, 10),
		Status: true,
	}

	sw.Ports["port1"] = port1
	sw.Ports["port2"] = port2

	// Start pipeline processing
	sw.Start()
	time.Sleep(100 * time.Millisecond)

	// Trigger mock frame ingestion
	dst, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	src, _ := net.ParseMAC("11:22:33:44:55:66")
	frame := &ethernet.Frame{
		Destination: dst,
		Source:      src,
		EtherType:   ethernet.EtherTypeIPv4,
		Payload:     []byte("test_payload"),
	}

	inFrame := dataplane.IncomingFrame{
		FRAME:   frame,
		IN_PORT: port1,
	}
	
	log.Println("Sending packet into dataPlaneChan")
	sw.dataPlaneChan <- inFrame

	// Verify port2 receives the flooded frame
	select {
	case out := <-port2.OutBuf:
		if string(out.Payload) != "test_payload" {
			t.Errorf("Unexpected payload received: %s", string(out.Payload))
		}
		log.Println("Successfully received packet out 2nd port!")
	case <-time.After(3 * time.Second):
		t.Errorf("Timeout waiting for packet to reach Port 2 out buffer")
	}

	// Since we mock, let's gracefully wait
	go sw.Stop()
	time.Sleep(100 * time.Millisecond)
}


