package l2

import (
	"log"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
)

func init() {
	FuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  IngressMacFilter,
		OutFunc: EgressMacFilter,
		Init:    InitMacFilter,
	}

	controlplane.RegisterLayerProc(2, "MACFilter", FuncPair)
}

type Filter struct {
	Mode int
}

type FilterRule struct {
	SrcAddress string
	DstAddress string
	Port       string // Optional
	Action     int    // 0 Allow, 1 Deny
}

type LocalMacAddress struct {
	Address string
}

type MACFilterConfig struct {
	IngressFilter  Filter
	EgressFilter   Filter
	IngressRule    []FilterRule
	EgressRule     []FilterRule
	LocalAddresses map[string]LocalMacAddress
}

const AllowAction int = 0
const DenyAction int = 1

func (conf *MACFilterConfig) GetIngressAction(src string, dst string, port string) int {
	/*
		action = IngressFilter.Mode

		for address in local addresses:
			if dst == address {
				return 0 (Permit)
			}

		for rule in ingress rule:
			MATCH = true
			if rule.src != "" & rule.src != src {
				MATCH = false
			}
			if rule.dst != "" & rule.dst != dst {
				MATCH = false
			}
			if rule.port != "" & rule.port != port {
				MATCH = false
			}
			if MATCH {
				action = rule.Action
				break
			}
	*/

	for _, addr := range conf.LocalAddresses {
		if dst == addr.Address {
			return AllowAction
		}
	}

	action := conf.IngressFilter.Mode

	for _, rule := range conf.IngressRule {
		MATCH := true
		if rule.SrcAddress != "" && rule.SrcAddress != src {
			MATCH = false
		}
		if rule.DstAddress != "" && rule.DstAddress != dst {
			MATCH = false
		}
		if rule.Port != "" && rule.Port != port {
			MATCH = false
		}
		if MATCH {
			action = rule.Action
			break
		}
	}

	return action
}

func (conf *MACFilterConfig) GetEgressAction(src string, dst string, port string) int {
	/*
		action = EgressFilter.Mode

		for address in local addresses:
			if dst == address {
				return 0 (Permit)
			}

		for rule in ingress rule:
			MATCH = true
			if rule.src != "" & rule.src != src {
				MATCH = false
			}
			if rule.dst != "" & rule.dst != dst {
				MATCH = false
			}
			if rule.port != "" & rule.port != port {
				MATCH = false
			}
			if MATCH {
				action = rule.Action
				break
			}
	*/
	for _, addr := range conf.LocalAddresses {
		if dst == addr.Address {
			return DenyAction
		}
	}

	action := conf.EgressFilter.Mode

	for _, rule := range conf.EgressRule {
		MATCH := true
		if rule.SrcAddress != "" && rule.SrcAddress != src {
			MATCH = false
		}
		if rule.DstAddress != "" && rule.DstAddress != dst {
			MATCH = false
		}
		if rule.Port != "" && rule.Port != port {
			MATCH = false
		}
		if MATCH {
			action = rule.Action
			break
		}
	}

	return action
}

func InitMacFilter(sw *controlplane.Switch) {
	log.Println("Starting MAC Filter Process")
	stor := sw.Stor.GetStor(2, "MACFilter")
	log.Printf("MAC Filter Process Config file path: %v", stor["ConfigFile"])
	configObj := MACFilterConfig{}
	if stor["ConfigFile"] != nil {
		path, ok := stor["ConfigFile"].(string)
		if ok {
			err := config.ReadConfigFile(path, &configObj)
			if err != nil {
				log.Printf("MAC Filter Failed to read config file due to error %v", err)
			} else {
				log.Printf("MAC Filter Config: %v", configObj)
				stor["CONFIG"] = configObj
			}
		} else {
			log.Printf("MAC Filter invalid config path specified")
		}
	}
}

func IngressMacFilter(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "MACFilter")
	configObj, ok := stor["CONFIG"].(MACFilterConfig)
	if !ok {
		log.Println("MAC Filter Config is not correct")
		return msg
	}

	frame := msgContent.InFrame.FRAME
	action := configObj.GetIngressAction(frame.Source.String(), frame.Destination.String(), msgContent.InFrame.IN_PORT.Name)
	log.Printf("Process MACFilter: Ingress Action=%v", action)
	if action == DenyAction {
		msg.Drop = true
	}
	return msg
}

func EgressMacFilter(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "MACFilter")
	configObj, ok := stor["CONFIG"].(MACFilterConfig)
	if !ok {
		log.Println("MAC Filter Config is not correct")
		return msg
	}

	outPorts := []*dataplane.SwitchPort{}
	frame := msgContent.InFrame.FRAME
	for _, port := range msgContent.OutPorts {
		action := configObj.GetEgressAction(frame.Source.String(), frame.Destination.String(), port.Name)
		log.Printf("Process MACFilter: Egress Action=%v for Port: %v", action, port.Name)
		if action == AllowAction {
			outPorts = append(outPorts, port)
		}
	}
	msgContent.OutPorts = outPorts

	return msg
}
