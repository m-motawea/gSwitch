package l3

import (
	"log"

	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/ip"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/ethernet"
)

func init() {
	FuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  IngressIpDecoder,
		OutFunc: EgressIpEncoder,
	}

	controlplane.RegisterLayerProc(3, "IPv4", FuncPair)
}

func IngressIpDecoder(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	if msgContent.InFrame.FRAME.EtherType != ethernet.EtherTypeIPv4 {
		// Accept only IPv4 for now
		msg.Finished = true
		return msg
	}
	payload, ok := msgContent.LayerPayload.([]byte)
	if !ok {
		log.Println("IP Process recieved invalid payload")
		msg.Drop = true
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
	msgContent.LayerPayload = ip4
	msg.Content = msgContent
	return msg
}

func EgressIpEncoder(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	if msgContent.InFrame.FRAME.EtherType != ethernet.EtherTypeIPv4 {
		// skip non IPv4 packets for now
		return msg
	}
	ip, ok := msgContent.LayerPayload.(ip.IPv4)
	if !ok {
		log.Println("IP Process Egress recieved invalid payload")
		msg.Drop = true
		return msg
	}
	payload, err := ip.MarshalBinary()
	if err != nil {
		log.Printf("IP Process egress failed to encode IP packet due to error %v", err)
		msg.Drop = true
		return msg
	}
	msgContent.LayerPayload = payload
	msgContent.InFrame.IN_PORT = nil
	msg.Content = msgContent
	return msg
}
