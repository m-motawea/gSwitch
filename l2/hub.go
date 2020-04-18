package l2

import (
	"log"

	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/pipeline"
)

func init() {
	HubProcFuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  HubInProc,
		OutFunc: HubOutProc,
		Init:    HubInitFunc,
	}

	controlplane.RegisterLayerProc(2, "Hub", HubProcFuncPair)
}

func HubInitFunc(sw *controlplane.Switch) {
	s := sw.Stor.GetStor(2, "Hub")
	s["number"] = 0
}

func HubInProc(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, ok := msg.Content.(controlplane.ControlMessage)
	if !ok {
		log.Println("Hub Proc Received Incompatible Message. Discarding..")
		msg.Drop = true
		return msg
	}
	log.Println("Hub Proc Received a Message.")
	for _, port := range msgContent.ParentSwitch.Ports {
		if port == msgContent.InFrame.IN_PORT {
			log.Println("Hub Proc Excluded IN PORT")
			continue
		}
		if !port.Status {
			continue
		}
		msgContent.OutPorts = append(msgContent.OutPorts, port)
	}
	msg.Content = msgContent
	return msg
}

func HubOutProc(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	return msg
}
