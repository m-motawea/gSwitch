package l2

import (
	"io/ioutil"
	"log"
	"net"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ethernet"
)

const ARP_EXPIRE_TIME = 60 * time.Second

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
	ent, ok := at.ARPTable[ipStr]
	if !ok {
		return nil
	}
	return ent
}

func (at *SwitchARPTable) DelEntry(ip net.IP) {
	ipStr := ip.String()
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
	at.ARPTable = map[string]*ARPEntry{}
	at.InverseARPTable = map[string][]*ARPEntry{}
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
	arpTable := SwitchARPTable{}
	arpTable.Init()
	stor["Table"] = arpTable
	go arpTable.CheckAndClear()
}

func ReplyARPIn(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	// This Process handles ARP requests destined to the switch and populate the ARP Table
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
		log.Println("ARP Process: this is arp frame.")
		p := new(arp.Packet)
		if err := p.UnmarshalBinary(frame.Payload); err != nil {
			log.Println("ARP Process Received invalid ARP Packet")
			return msg
		}

		stor := msgContent.ParentSwitch.Stor.GetStor(2, "ARP")
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
	log.Println("ARP Resolve proc...")
	return msg
}
