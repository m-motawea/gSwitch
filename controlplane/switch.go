package controlplane

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
)

type MACTable map[string]*dataplane.SwitchPort // mac address string to *port
type SwitchTable map[int]MACTable              // vlan id to mac table

type Switch struct {
	Name           string
	Ports          map[string]*dataplane.SwitchPort
	controlPipe    *pipeline.Pipeline
	Table          SwitchTable
	wg             *sync.WaitGroup
	dataPlaneChan  chan dataplane.IncomingFrame
	consumeChannel pipeline.PipelineChannel
	closeChan      chan int
}

func NewSwitch(name string, cfg config.Config, wg *sync.WaitGroup) *Switch {
	sw := Switch{}
	sw.initSwitch(name, cfg, wg)
	return &sw
}

func (sw *Switch) initSwitch(name string, cfg config.Config, wg *sync.WaitGroup) {
	log.Printf("intializing switch: %s", sw.Name)
	sw.Name = name
	sw.Table = SwitchTable{}
	sw.wg = wg
	sw.Ports = map[string]*dataplane.SwitchPort{}
	sw.dataPlaneChan = make(chan dataplane.IncomingFrame)
	sw.consumeChannel = make(pipeline.PipelineChannel)
	pipe, _ := pipeline.NewPipeline("ControlPlanePipeline", true, sw.wg, sw.consumeChannel)
	sw.controlPipe = &pipe
	// add pipeline processes
	for _, procConfig := range cfg.ControlProcess {
		// get the pair
		pair, ok := ControlProcs[procConfig.Layer][procConfig.Name]
		if !ok {
			log.Fatalf("No process in layer %d named %s", procConfig.Layer, procConfig.Name)
		}
		// create pipeline process
		procName := fmt.Sprintf("L%d:%s", procConfig.Layer, procConfig.Name)
		proc, err := pipeline.NewPipelineProcess(procName, pair.InFunc, pair.OutFunc)
		if err != nil {
			log.Fatalf("Failed to create process %s due to error %v", procName, err)
		}
		// add the process to the contolplane pipline
		sw.controlPipe.AddProcess(&proc)
	}
}

func (sw *Switch) AddSwitchPort(name string, swCfg config.SwitchPortConfig) (*dataplane.SwitchPort, error) {
	log.Printf("Switch %s: adding port %s", sw.Name, name)
	swPort, err := dataplane.NewSwitchPort(
		name,
		swCfg.Trunk,
		swCfg.AllowedVLANs...,
	)
	if err != nil {
		log.Printf("Switch %s: failed to add port %s due to erro %v", sw.Name, name, err)
		return &swPort, err
	}
	if swCfg.Up {
		swPort.Up(sw.dataPlaneChan)
	}
	sw.Ports[name] = &swPort
	return &swPort, nil
}

func (sw *Switch) DelSwitchPort(name string) {
	port, ok := sw.Ports[name]
	if !ok {
		log.Printf("No port named %s in switch %s!", name, sw.Name)
		return
	}
	if port.Status {
		port.Down()
	}
	delete(sw.Ports, name)
}

func (sw *Switch) SwitchLoop() {
	for {
		select {
		case <-sw.closeChan:
			return
		case inFrame := <-sw.dataPlaneChan:
			// incoming frames from ports
			ctrlMsg := ControlMessage{
				InFrame:      &inFrame,
				OutPorts:     []*dataplane.SwitchPort{},
				ParentSwitch: sw,
			}
			pipeMsg := pipeline.PipelineMessage{
				Direction: pipeline.PipelineInDirection{},
				Content:   ctrlMsg,
			}
			sw.controlPipe.SendMessage(pipeMsg)
			continue
		case pipeMsg := <-sw.consumeChannel:
			// processed msg from pipeline
			ctrlMsg, ok := pipeMsg.Content.(ControlMessage)
			if !ok {
				log.Fatal("Switch Loop Received Incompatible Message!")
			}
			for _, port := range ctrlMsg.OutPorts {
				port.Out(ctrlMsg.InFrame.FRAME)
			}
		}
	}
}

func (sw *Switch) Start() {
	go sw.controlPipe.Start()
	time.Sleep(5 * time.Second)
	go sw.SwitchLoop()
}

func (sw *Switch) Stop() {
	for _, port := range sw.Ports {
		port.Down()
	}
	sw.closeChan <- 1
}

func (sw *Switch) UpPort(name string) {
	port, ok := sw.Ports[name]
	if !ok {
		log.Printf("No port named %s in switch %s!", name, sw.Name)
		return
	}
	if port.Status {
		return
	}
	port.Up(sw.dataPlaneChan)
}

func (sw *Switch) DownPort(name string) {
	port, ok := sw.Ports[name]
	if !ok {
		log.Printf("No port named %s in switch %s!", name, sw.Name)
		return
	}
	if port.Status {
		port.Down()
	}
}
