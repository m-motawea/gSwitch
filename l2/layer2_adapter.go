package l2

import (
	"log"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/pipeline"
)

func init() {
	FuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  IngressAdapter,
		OutFunc: EgressAdapter,
		Init:    InitL2Adapter,
	}

	controlplane.RegisterLayerProc(2, "L2Adapter", FuncPair)
}

type ProxyAddress struct {
	Name string
	MAC  string
}

type L2AdapterConfig struct {
	AllowedAddresses map[string]ProxyAddress
}

func InitL2Adapter(sw *controlplane.Switch) {
	log.Println("Starting L2Adapter Process")
	stor := sw.Stor.GetStor(2, "L2Adapter")
	log.Printf("L2Adapter Process Config file path: %v", stor["ConfigFile"])
	configObj := L2AdapterConfig{}
	if stor["ConfigFile"] != nil {
		path, ok := stor["ConfigFile"].(string)
		if ok {
			err := config.ReadConfigFile(path, &configObj)
			if err != nil {
				log.Printf("L2Adapter Process Failed to read config file due to error %v", err)
			} else {
				log.Printf("L2Adapter Config: %+v", configObj)
				stor["CONFIG"] = configObj
			}
		} else {
			log.Printf("L2Adapter invalid config path specified")
		}
	}
}

func IngressAdapter(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	msgContent.PreMessage = msg
	msgContent.LayerPayload = msgContent.InFrame.FRAME.Payload
	log.Printf("L2 Adapter Ingress Next Layer Payload: %v", msgContent.LayerPayload)
	msg.Content = msgContent
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "L2Adapter")
	config, ok := stor["CONFIG"].(L2AdapterConfig)
	if !ok {
		log.Printf("L2 Adapter Config is not correct %+v", stor["CONFIG"])
		return msg
	}
	// if dst mac is not mine finish msg
	dstMACStr := msgContent.InFrame.FRAME.Destination.String()
	_, ok = config.AllowedAddresses[dstMACStr]
	if !ok {
		msg.Finished = true
	}
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
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "L2Adapter")
	config, ok := stor["CONFIG"].(L2AdapterConfig)
	if !ok {
		log.Printf("L2 Adapter Config is not correct %+v", stor["CONFIG"])
		return msg
	}
	// if dst mac is mine drop msg
	dstMACStr := msgContent.InFrame.FRAME.Destination.String()
	_, ok = config.AllowedAddresses[dstMACStr]
	if ok {
		msg.Drop = true
	}
	return msg
}
