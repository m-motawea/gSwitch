package controlplane

import (
	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
)

type ControlMessage struct {
	InFrame      *dataplane.IncomingFrame
	OutPorts     []*dataplane.SwitchPort
	ParentSwitch *Switch
}

type ControlProcessFuncPair struct {
	InFunc  func(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage
	OutFunc func(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage
}

var ControlProcs map[int]map[string]ControlProcessFuncPair
