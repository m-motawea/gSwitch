package l2

import (
	"io/ioutil"
	"log"
	"net"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

const ARP_EXPIRE_TIME = 60 * time.Second
const ARP_REQUEST_WAIT_TIME = 15 * time.Second

type LocalAddress struct {
	IP  string
	MAC string
}

type ARPConfig struct {
	LocalAddresses map[string]LocalAddress
}

type ARPEntry struct {
	IP            net.IP
	MAC           net.HardwareAddr
	Port          *dataplane.SwitchPort
	TimeCreated   time.Time
	LastRefreshed time.Time
}

func (ae *ARPEntry) Refresh() {
	ae.LastRefreshed = time.Now()
}

func (ae *ARPEntry) IsExpired() bool {
	return ae.LastRefreshed.Add(ARP_EXPIRE_TIME).Sub(time.Now()) < time.Second
}

type SwitchARPTable struct {
	ARPTable        map[string]*ARPEntry   // IP to one ARP Entry
	InverseARPTable map[string][]*ARPEntry // MAC to multiple ARP Entries
	rwMutex         *sync.RWMutex
}

func (at *SwitchARPTable) SetEntry(ip net.IP, mac net.HardwareAddr, port *dataplane.SwitchPort) *ARPEntry {
	log.Printf("ARP Process: Setting Entry for IP: %v, MAC: %v, PORT: %v", ip, mac, port)
	t := time.Now()
	ent := ARPEntry{
		IP:            ip,
		MAC:           mac,
		Port:          port,
		TimeCreated:   t,
		LastRefreshed: t,
	}
	strMAc := mac.String()

	defer at.rwMutex.Unlock()
	at.rwMutex.Lock()

	at.ARPTable[ip.String()] = &ent
	val, ok := at.InverseARPTable[strMAc]
	if !ok {
		val = []*ARPEntry{}
	}
	val = append(val, &ent)
	at.InverseARPTable[strMAc] = val
	return &ent
}

func (at *SwitchARPTable) GetEntry(ip net.IP) *ARPEntry {
	ipStr := ip.String()
	defer at.rwMutex.Unlock()
	at.rwMutex.Lock()
	ent, ok := at.ARPTable[ipStr]
	if !ok {
		return nil
	}
	return ent
}

func (at *SwitchARPTable) DelEntry(ip net.IP) {
	ipStr := ip.String()
	defer at.rwMutex.Unlock()
	at.rwMutex.Lock()
	ent, ok := at.ARPTable[ipStr]
	if !ok {
		return
	}
	mac := ent.MAC
	invMacList := at.InverseARPTable[mac.String()]

	var index int
	for i, invEnt := range invMacList {
		if ent == invEnt {
			index = i
			break
		}
	}
	invMacList = append(invMacList[:index], invMacList[index+1:]...)
	delete(at.ARPTable, ipStr)
}

func (at *SwitchARPTable) ClearExpired() {
	for _, ent := range at.ARPTable {
		log.Printf("ARP Process: Checking Entry %v", ent)
		if ent.IsExpired() {
			log.Printf("ARP Process: Entry %v Expired. Clearing..", ent)
			at.DelEntry(ent.IP)
		}
	}
}

func (at *SwitchARPTable) CheckAndClear() {
	log.Println("ARP Process: Starting ARP Table Check and Clear Routine..")
	for {
		timer := time.NewTimer(MAC_EXPIRE_TIME)
		<-timer.C
		at.ClearExpired()
		log.Printf("ARP Process: current ARP Table %v", at.ARPTable)
	}
}

func (at *SwitchARPTable) Init() {
	at.ARPTable = make(map[string]*ARPEntry)
	at.InverseARPTable = make(map[string][]*ARPEntry)
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
	arpTable := SwitchARPTable{rwMutex: &sync.RWMutex{}}
	arpTable.Init()
	stor["Table"] = arpTable
	go arpTable.CheckAndClear()
}

func ReplyARPIn(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	// This Process handles ARP requests destined to the switch and populate the ARP Table
	/*
		IF ARP:
			IF ARP Reply:
				- Add Target IP, MAC & Port to ARP Table
				IF Sender IP is a Local IP (defined in config file):
					- Drop Message
			Else IF ARP Request:
				- Add Source IP, MAC & PORT to ARP Table
				IF Target IP is a Local IP (defined in config file):
					- Reply
	*/

	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "ARP")
	config, ok := stor["CONFIG"].(ARPConfig)
	if !ok {
		log.Println("ARP Config is not correct")
		return msg
	}

	frame := msgContent.InFrame.FRAME
	log.Printf("ARP Reply proc Frame of Type %v", frame.EtherType.String())
	if frame.EtherType == ethernet.EtherTypeARP {
		log.Println("ARP Process: this is arp frame.")
		p := new(arp.Packet)
		if err := p.UnmarshalBinary(frame.Payload); err != nil {
			log.Println("ARP Process Received invalid ARP Packet")
			return msg
		}

		table := stor["Table"].(SwitchARPTable)
		targetIP := p.TargetIP.String()

		if p.Operation == arp.OperationReply {
			log.Println("ARP Process: This is ARP Reply")
			// ARP Reply
			// Add target ip and mac to ARP Table
			table.SetEntry(p.TargetIP, p.TargetHardwareAddr, msgContent.InFrame.IN_PORT)
			// if it is for a local address drop
			for _, addr := range config.LocalAddresses {
				if addr.IP == targetIP {
					log.Printf("ARP Process: This is ARP Reply to My IP: %v", addr.IP)
					msg.Drop = true
				}
			}
			return msg
		} else {
			// ARP Request
			// Add Sender MAC and IP to ARP table
			log.Println("ARP Process: This is ARP Request")
			table.SetEntry(p.SenderIP, p.SenderHardwareAddr, msgContent.InFrame.IN_PORT)
		}

		log.Printf("ARP Process: Target IP: %s", targetIP)

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
				msg.Finished = true
				log.Printf("ARP Process sending result")
				return msg
			}
		}
		log.Printf("ARP Process: IP %s not a local address", targetIP)
	}
	return msg
}

func ResolveARPOut(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	// This Processes sets the appropriate SRC and DST MAC Address for all IPv4 internal frames
	log.Println("ARP Resolve proc...")
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	if msgContent.InFrame.FRAME.EtherType != ethernet.EtherTypeIPv4 {
		return msg
	}
	log.Println("ARP Process: Received IPv4 Frame")

	ipPayload := msgContent.InFrame.FRAME.Payload
	if len(ipPayload) < 20 {
		// invalid IPv4 payload
		log.Printf("ARP Process got invalid IPv4 payload")
		msg.Drop = true
		return msg
	}
	dstIP := net.IP(ipPayload[12:16])
	srcIP := net.IP(ipPayload[16:20])
	log.Printf("ARP Process: SRC IP: %v, DST IP: %v", srcIP, dstIP)
	// if srcIP is mine set src mac and send ARP Request to get destination mac
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "ARP")
	config, ok := stor["CONFIG"].(ARPConfig)
	if !ok {
		log.Println("ARP Config is not correct")
		return msg
	}

	srcIPStr := srcIP.String()
	table := stor["Table"].(SwitchARPTable)
	for iface, addr := range config.LocalAddresses {
		if addr.IP == srcIPStr {
			// Send ARP and wait for result before setting the destination mac
			log.Printf("ARP Process: Setting Proper Source & Destination MAC for interface %v", iface)
			byteMAC, err := net.ParseMAC(addr.MAC)
			if err != nil {
				log.Printf("ARP Process: Failed to parse mac addr of local address %v", addr)
				msg.Drop = true
				return msg
			}
			msgContent.InFrame.FRAME.Source = net.HardwareAddr(byteMAC)
			// Search for ARP Entry for the destination IP
			ent := table.GetEntry(dstIP)
			if ent != nil {
				log.Printf("ARP Process: Found ARP Entry for IP: %v, MAC: %v", dstIP, ent.MAC)
				msgContent.InFrame.FRAME.Destination = ent.MAC
				msg.Content = msgContent
				return msg
			}
			log.Printf("ARP Process: Couldn't Find ARP Entry for IP: %v. trying to resolve it...", dstIP)
			dstMac := ResolveIP(srcIP, dstIP, byteMAC, msgContent.ParentSwitch, table)
			if dstMac == nil {
				log.Printf("ARP Process: Unable to resolve IP: %v", dstIP)
				msg.Drop = true
				return msg
			}
			msgContent.InFrame.FRAME.Destination = *dstMac
			msg.Content = msgContent
			return msg
		}
	}
	return msg
}

func ResolveIP(srcIP net.IP, dstIP net.IP, srcMAC net.HardwareAddr, sw *controlplane.Switch, table SwitchARPTable) *net.HardwareAddr {
	// build arp frame
	p, err := arp.NewPacket(
		arp.OperationRequest,
		srcMAC,
		srcIP,
		ethernet.Broadcast,
		dstIP,
	)
	if err != nil {
		log.Printf("ARP Process: Failed to build ARP Request due to error %v", err)
		return nil
	}
	pb, err := p.MarshalBinary()
	if err != nil {
		log.Printf("ARP Process: Failed to Marshal ARP Request due to error %v", err)
		return nil
	}
	f := &ethernet.Frame{
		Destination: ethernet.Broadcast,
		Source:      srcMAC,
		EtherType:   ethernet.EtherTypeARP,
		Payload:     pb,
	}
	// send it out of all switch ports
	ports := []*dataplane.SwitchPort{}
	for _, p := range sw.Ports {
		ports = append(ports, p)
	}
	go sw.SendFrame(f, ports...)
	// check switch ARP table until timeout
	timeout := time.Now().Add(ARP_REQUEST_WAIT_TIME)
	for {
		if time.Now().Sub(timeout) > time.Second {
			log.Printf("ARP Process: ARP Request for IP: %v Timedout.", dstIP)
			return nil
		}
		ent := table.GetEntry(dstIP)
		if ent != nil {
			return &ent.MAC
		}
	}
}
