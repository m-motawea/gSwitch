package l3

import (
	"log"

	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/pipeline"
	"github.com/m-motawea/ip"
	"github.com/mdlayher/ethernet"
)

func init() {
	FuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  IngressIpEncoder,
		OutFunc: controlplane.DummyProc,
	}

	controlplane.RegisterLayerProc(3, "IPv4", FuncPair)
}

func IngressIpEncoder(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	payload := msgContent.LayerPayload
	if msgContent.InFrame.FRAME.EtherType != ethernet.EtherTypeIPv4 {
		// Accept only IPv4 for now
		msg.Finished = true
		return msg
	}
	ip4 := ip.IPv4{}
	err := ip4.UnmarshalBinary(payload)
	if err != nil {
		log.Printf("IP process Failed to marshal payload due to error %v.\nPayload: %v", err, payload)
		msg.Finished = true
		return msg
	}
	log.Printf("IP Process: decoded IPv4: %+v", ip4)
	return msg
}
