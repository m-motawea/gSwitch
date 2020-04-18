package l2

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/ethernet"
)

const MAC_EXPIRE_TIME = 30 * time.Second

type MACEntry struct {
	Addr          net.HardwareAddr
	Port          *dataplane.SwitchPort
	TimeCreated   time.Time
	LastRefreshed time.Time
}

func (me *MACEntry) Refresh() {
	me.LastRefreshed = time.Now()
}

func (me *MACEntry) IsExpired() bool {
	return me.LastRefreshed.Add(MAC_EXPIRE_TIME).Sub(time.Now()) < time.Second
}

type MACTable struct {
	Table map[string]*MACEntry
	VLAN  int
}

func (mt *MACTable) Init() {
	mt.Table = map[string]*MACEntry{}
}

func (mt *MACTable) GetEntry(addr string) *MACEntry {
	ent, ok := mt.Table[addr]
	if !ok {
		return nil
	}
	return ent
}

func (mt *MACTable) SetEntry(addr string, port *dataplane.SwitchPort) *MACEntry {
	ent := mt.GetEntry(addr)
	if ent != nil {
		ent.Port = port
		ent.Refresh()
		return ent
	}
	t := time.Now()
	ent = &MACEntry{
		Addr:          net.HardwareAddr(addr),
		Port:          port,
		TimeCreated:   t,
		LastRefreshed: t,
	}
	mt.Table[addr] = ent
	fmt.Printf("Entry set %v", ent)
	return ent
}

func (mt *MACTable) DelEntry(addr string) {
	delete(mt.Table, addr)
}

func (mt *MACTable) Clear() {
	for addr := range mt.Table {
		mt.DelEntry(addr)
	}
}

func (mt *MACTable) ClearExpired() {
	for addr, ent := range mt.Table {
		if ent.IsExpired() {
			mt.DelEntry(addr)
		}
	}
}

type SwitchMACTable map[int]*MACTable

func (st SwitchMACTable) GetVlanEntry(vlan int, addr string) *MACEntry {
	vlanTable, ok := st[vlan]
	if !ok {
		st[vlan] = &MACTable{VLAN: vlan}
		st[vlan].Init()
		return nil
	}
	return vlanTable.GetEntry(addr)
}

func (st SwitchMACTable) GetOutPort(frame *ethernet.Frame, sw *controlplane.Switch) []*dataplane.SwitchPort {
	log.Println("=============================================================================================")
	outPorts := []*dataplane.SwitchPort{}
	vlanObj := frame.VLAN
	vlan := 0
	if vlanObj != nil {
		vlan = int(vlanObj.ID)
	}
	addr := frame.Destination.String()
	log.Printf("Getting OutPort for addr %s, vlan %d", addr, vlan)
	entry := st.GetVlanEntry(vlan, addr)
	if entry != nil {
		outPorts = append(outPorts, entry.Port)
	} else {
		log.Println("Couldn't find Entry!")
		for _, port := range sw.Ports {
			outPorts = append(outPorts, port)
		}
	}
	log.Printf("Out Ports: %v", outPorts)
	log.Println("=============================================================================================")
	return outPorts
}

func (st SwitchMACTable) SetInPort(frame *ethernet.Frame, inPort *dataplane.SwitchPort) *MACEntry {
	log.Println("=============================================================================================")
	log.Println("Setting MAC Entry for in Frame")
	log.Println("=============================================================================================")
	vlanObj := frame.VLAN
	vlan := 0
	if vlanObj != nil {
		vlan = int(vlanObj.ID)
	}
	addr := frame.Source.String()
	vlanTable, ok := st[vlan]
	if !ok {
		vlanTable = &MACTable{}
		st[vlan] = vlanTable
		st[vlan].Init()
	}
	return vlanTable.SetEntry(addr, inPort)
}

func (st SwitchMACTable) CheckAndClearLoop() {
	for {
		timer := time.NewTimer(MAC_EXPIRE_TIME)
		<-timer.C
		for _, t := range st {
			t.ClearExpired() // TODO use go?
		}
	}
}

func init() {
	L2SwitchProcFuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  L2SwitchInFunc,
		OutFunc: L2SwitchOutFunc,
		Init:    InitL2Switch,
	}

	controlplane.RegisterLayerProc(2, "L2Switch", L2SwitchProcFuncPair)
}

func InitL2Switch(sw *controlplane.Switch) {
	st := SwitchMACTable{}
	stor := sw.Stor.GetStor(2, "L2Switch")
	stor["SwitchTable"] = st
	go st.CheckAndClearLoop()
}

func L2SwitchInFunc(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	// This process is used to populate the SwitchMACTable Only
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "L2Switch")
	st := stor["SwitchTable"].(SwitchMACTable)

	inPort := msgContent.InFrame.IN_PORT
	frame := msgContent.InFrame.FRAME
	st.SetInPort(frame, inPort)
	return msg
}

func L2SwitchOutFunc(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	// Selection Process for out ports
	msgContent, _ := msg.Content.(controlplane.ControlMessage)
	stor := msgContent.ParentSwitch.Stor.GetStor(2, "L2Switch")
	st := stor["SwitchTable"].(SwitchMACTable)
	frame := msgContent.InFrame.FRAME
	outPorts := st.GetOutPort(frame, msgContent.ParentSwitch)

	msgContent.OutPorts = outPorts
	msg.Content = msgContent
	return msg
}