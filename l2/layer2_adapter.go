package l2

import (
	"log"

	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/ip"
	"github.com/m-motawea/pipeline"
)

func init() {
	FuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  IngressAdapter,
		OutFunc: EgressAdapter,
	}

	controlplane.RegisterLayerProc(2, "L2Adapter", FuncPair)
}

func IngressAdapter(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	msgContent.PreMessage = msg
	msgContent.LayerPayload = msgContent.InFrame.FRAME.Payload
	log.Printf("L2 Adapter Ingress Next Layer Payload: %v", msgContent.LayerPayload)
	msg.Content = msgContent
	return msg
}

func EgressAdapter(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	lp, ok := msgContent.LayerPayload.([]byte)
	if !ok {
		log.Println("L2 Adapter Egress recieved invalid payload from previous process")
		msg.Drop = true
		return msg
	}
	if len(lp) > 0 {
		msgContent.InFrame.FRAME.Payload = lp
	}
	// msg.Content = *msgContent.PreMessage
	msg.Content = msgContent
	i := ip.IPv4{}
	_ = i.UnmarshalBinary(lp)
	log.Printf("L@ Adapter IPv4 %+v", i)
	return msg
}
