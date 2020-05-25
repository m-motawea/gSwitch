package controlplane

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/m-motawea/gSwitch/config"
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
	"github.com/mdlayher/ethernet"
)

type ProcStor map[string]interface{} // Key to Val

type LayerStor map[string]ProcStor // Proc Name to ProcStro

type SwitchProcStor map[int]LayerStor // Layer Number to LayerStor

func (swStor SwitchProcStor) getLayerStor(layer int) LayerStor {
	ls, ok := swStor[layer]
	if !ok {
		ls = LayerStor{}
		swStor[layer] = ls
	}
	return ls
}

func (ls LayerStor) getProcStor(name string) ProcStor {
	ps, ok := ls[name]
	if !ok {
		ps = ProcStor{}
		ls[name] = ps
	}
	return ps
}

func (swStor SwitchProcStor) GetStor(layer int, name string) ProcStor {
	ls := swStor.getLayerStor(layer)
	ps := ls.getProcStor(name)
	return ps
}

type Switch struct {
	Name           string
	Ports          map[string]*dataplane.SwitchPort
	Stor           SwitchProcStor
	controlPipe    *pipeline.Pipeline
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
	sw.wg = wg
	sw.Stor = SwitchProcStor{}
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
		if pair.Init != nil {
			stor := sw.Stor.GetStor(procConfig.Layer, procConfig.Name)
			stor["ConfigFile"] = procConfig.ConfigFile
			pair.Init(sw)
		}
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
	go sw.ConsumerLoop()
	for {
		select {
		case <-sw.closeChan:
			log.Println("Control Plane: stopping SwitchLoop..")
			return
		case inFrame := <-sw.dataPlaneChan:
			// incoming frames from ports
			log.Println("Control Plane: received dataplane message. sending to pipline...")
			ctrlMsg := ControlMessage{
				InFrame:      &inFrame,
				OutPorts:     []*dataplane.SwitchPort{},
				ParentSwitch: sw,
				LayerPayload: []byte{},
			}
			pipeMsg := pipeline.PipelineMessage{
				Direction: pipeline.PipelineInDirection{},
				Content:   ctrlMsg,
			}
			sw.controlPipe.SendMessage(pipeMsg)
			log.Println("Control Plane: message sent to pipeline")
			continue
		}
	}
}

func (sw *Switch) ConsumerLoop() {
	for {
		select {
		case <-sw.closeChan:
			log.Println("Control Plane: stopping ConsumerLoop..")
			return
		case pipeMsg := <-sw.consumeChannel:
			// processed msg from pipeline
			log.Println("Control Plane: received pipeline message. sending out to dataplane...")
			ctrlMsg, ok := pipeMsg.Content.(ControlMessage)
			if !ok {
				log.Fatal("Switch Loop Received Incompatible Message!")
			}
			for _, port := range ctrlMsg.OutPorts {
				log.Printf("Control Plane: sending msg to port %s...", port.Name)
				port.Out(ctrlMsg.InFrame.FRAME)
				log.Printf("Control Plane: msg send to port %s.", port.Name)
			}
			log.Println("Control Plane: message sent to dataplane")
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
	sw.controlPipe.Stop()
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

func (sw *Switch) SendFrame(frame *ethernet.Frame, OutPorts ...*dataplane.SwitchPort) {
	if len(OutPorts) == 0 {
		return
	}
	for _, port := range OutPorts {
		log.Printf("Switch: Async Frame output to port %s...", port.Name)
		port.Out(frame)
		log.Printf("Switch: Async Frame output to port %s.", port.Name)
	}
}
