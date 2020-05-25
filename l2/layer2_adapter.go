package l2

import (
	"log"

	"github.com/m-motawea/gSwitch/controlplane"
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
	msgContent.LayerPayload = msgContent.InFrame.FRAME.Payload
	log.Printf("L2 Adapter Ingress Next Layer Payload: %v", msgContent.LayerPayload)
	msg.Content = msgContent
	return msg
}

func EgressAdapter(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	if len(msgContent.LayerPayload) > 0 {
		msgContent.InFrame.FRAME.Payload = msgContent.LayerPayload
	}
	// msg.Content = *msgContent.PreMessage
	log.Printf("L2 Adapter Egress Previous Layer Payload: %v", msgContent.LayerPayload)
	msg.Content = msgContent
	return msg
}
