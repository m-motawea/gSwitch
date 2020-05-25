package l3

import (
	"log"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/pipeline"
)

type LocalAddress struct {
	Address string
}

type ICMPConfig struct {
	LocalAddresses []LocalAddress
}

func InitICMP(sw *controlplane.Switch) {
	log.Println("Starting MAC Filter Process")
	stor := sw.Stor.GetStor(3, "ICMP")
	log.Printf("ICMP Filter Process Config file path: %v", stor["ConfigFile"])
	configObj := ICMPConfig{}
	if stor["ConfigFile"] != nil {
		path, ok := stor["ConfigFile"].(string)
		if ok {
			err := config.ReadConfigFile(path, &configObj)
			if err != nil {
				log.Printf("ICMP Failed to read config file due to error %v", err)
			} else {
				log.Printf("ICMP Config: %v", configObj)
				stor["CONFIG"] = configObj
			}
		} else {
			log.Printf("ICMP invalid config path specified")
		}
	}
}

func ICMPProcessIn(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "MACFilter")
	_, ok := stor["CONFIG"].(ICMPConfig)
	if !ok {
		log.Println("ICMP Config is not correct")
		return msg
	}
	return msg
}
