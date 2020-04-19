package l2

import (
	"io/ioutil"
	"log"
	"net"

	"github.com/BurntSushi/toml"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

type LocalAddress struct {
	IP  string
	MAC string
}

type ARPConfig struct {
	LocalAddresses map[string]LocalAddress
}

func readConfig(path string) (ARPConfig, error) {
	confBin, err := ioutil.ReadFile(path)
	var config ARPConfig
	if err != nil {
		log.Println("ARP Process failed to open config file: ", path)
		return config, err
	}
	confStr := string(confBin)
	_, err = toml.Decode(confStr, &config)
	if err != nil {
		log.Printf("ARP Process failed to decode config: %s due to error %v", path, err)
		return config, err
	}
	return config, nil
}

func init() {
	ARPProcFuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  ReplyARPIn,
		OutFunc: ResolveARPOut,
		Init:    InitARP,
	}

	controlplane.RegisterLayerProc(2, "ARP", ARPProcFuncPair)
}

func InitARP(sw *controlplane.Switch) {
	log.Println("Starting ARP Process")
	stor := sw.Stor.GetStor(2, "ARP")
	log.Printf("ARP Process Config file path: %v", stor["ConfigFile"])
	if stor["ConfigFile"] == nil {
		stor["CONFIG"] = ARPConfig{
			LocalAddresses: map[string]LocalAddress{},
		}
	} else {
		path, ok := stor["ConfigFile"].(string)
		if !ok {
			stor["CONFIG"] = ARPConfig{
				LocalAddresses: map[string]LocalAddress{},
			}
		} else {
			conf, err := readConfig(path)
			if err != nil {
				stor["CONFIG"] = ARPConfig{
					LocalAddresses: map[string]LocalAddress{},
				}
			} else {
				stor["CONFIG"] = conf
			}
		}
	}
}

func ReplyARPIn(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	// This Process handles ARP requests destined to the switch and populate the ARP Table
	// TODO Populate ARP Table of all L2 Frames
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "ARP")
	config, ok := stor["CONFIG"].(ARPConfig)
	if !ok {
		log.Println("ARP Config is not correct")
		return msg
	}
	log.Printf("ARPConfig: %v", config)

	frame := msgContent.InFrame.FRAME
	log.Printf("ARP Reply proc Frame of Type %v", frame.EtherType.String())
	if frame.EtherType == ethernet.EtherTypeARP {
		log.Println("ARP Reply: this is arp request.")
		p := new(arp.Packet)
		if err := p.UnmarshalBinary(frame.Payload); err != nil {
			log.Println("ARP Process Received invalid ARP Packet")
			return msg
		}
		targetIP := p.TargetIP.String()
		log.Printf("ARP Process: ARP Request for IP: %s", targetIP)
		for iface, addr := range config.LocalAddresses {
			log.Printf("ARP Process: checking addr: %s", addr.IP)
			mac, err := net.ParseMAC(addr.MAC)
			if err != nil {
				log.Printf("ARP Process: invalid local mac %s", addr.MAC)
				msg.Drop = true
				return msg
			}
			if addr.IP == targetIP {
				log.Printf("ARP Process replying with address of %s", iface)
				replyPacket, err := arp.NewPacket(
					arp.OperationReply,
					mac,
					net.ParseIP(addr.IP),
					frame.Source,
					net.ParseIP(targetIP),
				)
				if err != nil {
					log.Printf("ARP Process: Failed to create the reply packet err: %v", err)
					msg.Drop = true
					return msg
				}

				pb, err := replyPacket.MarshalBinary()
				if err != nil {
					log.Printf("ARP Process: Failed to marshal reply packet err: %v", err)
					msg.Drop = true
					return msg
				}
				f := &ethernet.Frame{
					Destination: frame.Source,
					Source:      replyPacket.SenderHardwareAddr,
					EtherType:   ethernet.EtherTypeARP,
					Payload:     pb,
					VLAN:        frame.VLAN,
				}
				log.Printf("ARP Process preparing result msg....")
				msgContent.InFrame.FRAME = f
				msgContent.InFrame.IN_PORT = &dataplane.SwitchPort{}
				msg.Content = msgContent
				// msg.Finished = true
				log.Printf("ARP Process sending result")
				return msg
			}
		}
		log.Printf("ARP Process: IP %s not a local address", targetIP)
	}
	return msg
}

func ResolveARPOut(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	log.Println("ARP Resolve proc...")
	return msg
}
