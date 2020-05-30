package l3

import (
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/ip"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/ethernet"
)

func init() {
	FuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  IngressRouting,
		OutFunc: EgressRouting,
		Init:    InitRouting,
	}

	controlplane.RegisterLayerProc(3, "Routing", FuncPair)
}

type Port struct {
	Name    string
	NextHop string
}

type Route struct {
	Ports []Port
}

type VLANIface struct {
	IP   string
	MAC  string
	VLAN uint16
}

type RoutingTable struct {
	VLANIfaces map[string]VLANIface
	Routes     map[string]Route
}

func InitRouting(sw *controlplane.Switch) {
	log.Println("Starting Routing Process")
	stor := sw.Stor.GetStor(3, "Routing")
	log.Printf("Routing Process Config file path: %v", stor["ConfigFile"])
	configObj := RoutingTable{}
	if stor["ConfigFile"] != nil {
		path, ok := stor["ConfigFile"].(string)
		if ok {
			err := config.ReadConfigFile(path, &configObj)
			if err != nil {
				log.Printf("Routing Process Failed to read config file due to error %v", err)
			} else {
				log.Printf("Routing Config: %+v", configObj)
				stor["CONFIG"] = configObj
			}
		} else {
			log.Printf("Routing invalid config path specified")
		}
	}
}

func IngressRouting(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	/*
		if msg ip.destination in my addresses > continue the pipeline
		else if msg frame.destination in my addresses > route the message
		else drop the message

	*/
	log.Println("Routing Process: Ingress Recieved a message")
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(3, "Routing")
	config, ok := stor["CONFIG"].(RoutingTable)
	if !ok {
		log.Printf("Routing Config is not correct %+v", stor["CONFIG"])
		return msg
	}
	i, _ := msgContent.LayerPayload.(ip.IPv4)
	dstIP := i.Destination.String()
	dstMAC := msgContent.InFrame.FRAME.Destination.String()
	log.Printf("Routing Process: dstIP %s, dstMAC: %s", dstIP, dstMAC)
	for _, addr := range config.VLANIfaces {
		if addr.IP == dstIP {
			log.Printf("Routing: Ingress packet sent to my address %s", addr.IP)
			msg.Finished = false
			return msg
		} else if dstMAC == addr.MAC {
			msg.Finished = true
		}
	}

	if !msg.Finished {
		msg.Drop = true
	}

	return msg
}

func EgressRouting(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	/*
		loop on all routes in config
		split key to get network and mask
		if ip.dst & addr.MASK == network & MASK {
			- set source mac address as addr.MAC
			- set frame vlan as addr.VLAN
		}
	*/
	log.Println("Routing Process: Egress Recieved a message")
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(3, "Routing")
	config, ok := stor["CONFIG"].(RoutingTable)
	if !ok {
		log.Printf("Routing Config is not correct %+v", stor["CONFIG"])
		return msg
	}
	i, _ := msgContent.LayerPayload.(ip.IPv4)
	dstIPStr := i.Destination.String()
	// make sure dstIP is not me
	for _, localIface := range config.VLANIfaces {
		if localIface.IP == dstIPStr {
			log.Printf("Routing Process: Egress Dropping payload with destination as %s", dstIPStr)
			msg.Drop = true
			return msg
		}
	}

	dstMAC := msgContent.InFrame.FRAME.Destination.String()
	log.Printf("Routing Process: dstIP %s, dstMAC: %s", dstIPStr, dstMAC)
	for prefix, route := range config.Routes {
		log.Printf("Rouing Process: comparing prefix: %s, with dest %s", prefix, dstIPStr)
		temp := strings.Split(prefix, "/")
		if len(temp) != 2 {
			log.Printf("Routing Process: Invalid prefix in routing table %s", prefix)
			continue
		}
		networkAddr := ip.IP(0)
		err := networkAddr.FromString(temp[0])
		log.Printf("Routing Process network %s, IP: %d", temp[0], networkAddr)
		if err != nil {
			log.Printf("Routing Process: Invalid prefix in routing table %s", prefix)
			continue
		}
		cidr, err := strconv.Atoi(temp[1])
		mask := uint32(0xFFFFFFFF << (32 - cidr))
		log.Printf("Routing Process Mask is %x", mask)
		if err != nil {
			log.Printf("Routing Process: Invalid prefix in routing table %s", prefix)
			continue
		}
		networkMatch := uint32(networkAddr) & mask
		log.Printf("Routing Process: Prefix stored uint32 %d", uint32(networkAddr))
		destMatch := uint32(i.Destination) & mask
		log.Printf("Rouing Process: comparing network: %d, with dest %d", networkMatch, destMatch)
		if networkMatch == destMatch {
			// use only one port for now
			log.Printf("Route %s matched destination", prefix)
			if len(route.Ports) < 1 {
				log.Printf("Routing Process: no ports in this route for prefix %s", prefix)
				continue
			}
			port := route.Ports[0]
			iface, ok := config.VLANIfaces[port.Name]
			if !ok {
				log.Printf("Routing Process: no VLANIface named %s", port.Name)
				continue
			}
			srcMAC, err := net.ParseMAC(iface.MAC)
			if err != nil {
				log.Printf("Routing Process: Invalid MAC Address in interface %s", port.Name)
			}
			msgContent.InFrame.FRAME.VLAN = &ethernet.VLAN{ID: iface.VLAN}
			msgContent.InFrame.FRAME.Source = srcMAC
			msgContent.InFrame.FRAME.Destination = nil
			msg.Content = msgContent
			log.Printf("Routing Proc: out frame %+v", msgContent.InFrame.FRAME)
			return msg
		}
	}
	log.Printf("Routing Process: no match for frame %+v", msgContent.InFrame.FRAME)
	return msg
}
