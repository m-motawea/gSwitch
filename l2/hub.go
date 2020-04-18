package l2

import (
	"log"

	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/pipeline"
)

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
