package l3

import (
	"log"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/icmp"
	"github.com/m-motawea/ip"
	"github.com/m-motawea/pipeline"
)

type LocalAddress struct {
	Address string
}

type ICMPConfig struct {
	LocalAddresses map[string]LocalAddress
}

func init() {
	FuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  ICMPProcessIn,
		OutFunc: controlplane.DummyProc,
		Init:    InitICMP,
	}

	controlplane.RegisterLayerProc(3, "ICMP", FuncPair)
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
	stor := msgContent.ParentSwitch.Stor.GetStor(3, "ICMP")
	config, ok := stor["CONFIG"].(ICMPConfig)
	if !ok {
		log.Printf("ICMP Config is not correct %+v", stor["CONFIG"])
		return msg
	}
	i, _ := msgContent.LayerPayload.(ip.IPv4)
	if i.Protocol != ip.PROTO_ICMP {
		log.Printf("ICMP Process not icmp protocol %+v", i.Protocol)
		return msg
	}
	// 1- check if i.Destination matches one of my VLAN interfaces in config
	// 2- if matches: load ICMP packet and check if it is an ICMP request
	// 3- if icmp request create a reply
	for key, val := range config.LocalAddresses {
		if i.Destination.String() == val.Address {
			log.Printf("ICMP Proc: ip destination is my interface: %s, IP: %s", key, val.Address)
			ic := icmp.ICMP{}
			err := ic.UnmarshalBinary(i.Data)
			if err != nil {
				log.Printf("ICMP failed to unmarshal icmp data due to error %v", err)
				msg.Drop = true
				return msg
			}
			log.Printf("icmp: %+v", ic)
			if ic.Type == icmp.TYPE_ICMP_ECHO_REPQUEST {
				dst := i.Source
				src := i.Destination
				i.Source = src
				i.Destination = dst
				i.TTL = ip.TTL(64)
				ic.Type = icmp.TYPE_ICMP_ECHO_REPLY
				data, err := ic.MarshalBinary()
				if err != nil {
					log.Printf("Failed to marshal resposne icmp due to error %v", err)
					msg.Drop = true
					return msg
				}
				i.Data = data
				msgContent.LayerPayload = i
				msg.Content = msgContent
				log.Printf("ICMP Proc result ICMP: %+v", ic)
				log.Printf("ICMP Proc result IP: %+v", i)
				return msg
			} else {
				msg.Drop = true
				return msg
			}
		}
	}
	return msg
}
