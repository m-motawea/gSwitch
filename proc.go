package main

import (
	"github.com/m-motawea/gSwitch/controlplane"
	"github.com/m-motawea/gSwitch/l2"
)

func RegisterProcs() {
	controlplane.ControlProcs = map[int]map[string]controlplane.ControlProcessFuncPair{}
	HubProcFuncPair := controlplane.ControlProcessFuncPair{
		InFunc:  l2.HubInProc,
		OutFunc: l2.HubOutProc,
	}

	registerLayerProc(2, "Hub", HubProcFuncPair)
}

func registerLayerProc(layer int, name string, pair controlplane.ControlProcessFuncPair) {
	_, ok := controlplane.ControlProcs[layer]
	if !ok {
		controlplane.ControlProcs[layer] = map[string]controlplane.ControlProcessFuncPair{}
	}
	controlplane.ControlProcs[layer][name] = pair
}
