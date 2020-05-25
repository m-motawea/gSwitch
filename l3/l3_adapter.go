package l3

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

	controlplane.RegisterLayerProc(3, "L3Adapter", FuncPair)
}

func IngressAdapter(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	newmsg := pipeline.PipelineMessage{}
	newmsg = msg
	msgContent, _ := newmsg.Content.(controlplane.ControlMessage)
	ip, ok := msgContent.LayerPayload.(ip.IPv4)
	if !ok {
		log.Println("IP Process Ingress recieved invalid payload")
		msg.Drop = true
		return msg
	}
	msgContent.PreMessage = msg
	msgContent.LayerPayload = ip.Data
	newmsg.Content = msgContent
	log.Printf("L3 Adapter Ingress Msg %+v", newmsg)
	return newmsg
}

func EgressAdapter(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	premsg, ok := msgContent.PreMessage.(pipeline.PipelineMessage)
	if !ok {
		log.Println("L3 Adapter Egress Recieved invalid message")
		msg.Drop = true
		return msg
	}
	premsgContent, _ := premsg.Content.(controlplane.ControlMessage)
	ip, ok := premsgContent.LayerPayload.(ip.IPv4)
	if !ok {
		log.Println("IP Process Egress recieved invalid premsg payload")
		msg.Drop = true
		return msg
	}
	payload, ok := msgContent.LayerPayload.([]byte)
	if !ok {
		log.Println("IP Process Egress recieved invalid payload")
		msg.Drop = true
		return msg
	}
	ip.Data = payload
	premsgContent.LayerPayload = ip
	msg.Content = premsgContent
	log.Printf("L3 Adapter Egress Msg %+v", msg)
	return msg
}
