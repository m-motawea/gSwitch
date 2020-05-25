package controlplane

import (
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
)

type ControlMessage struct {
	InFrame *dataplane.IncomingFrame
	// PreMessage   *ControlMessage // To be able to reconstruct the packet again
	LayerPayload []byte // To separate each leayer payload
	OutPorts     []*dataplane.SwitchPort
	ParentSwitch *Switch
}

type ControlProcessFuncPair struct {
	InFunc  func(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage
	OutFunc func(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage
	Init    func(sw *Switch)
}

var ControlProcs map[int]map[string]ControlProcessFuncPair

func init() {
	ControlProcs = map[int]map[string]ControlProcessFuncPair{}
}

func RegisterLayerProc(layer int, name string, pair ControlProcessFuncPair) {
	_, ok := ControlProcs[layer]
	if !ok {
		ControlProcs[layer] = map[string]ControlProcessFuncPair{}
	}
	ControlProcs[layer][name] = pair
}

func DummyProc(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage {
	return msg
}
