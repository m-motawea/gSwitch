package controlplane

import (
	"encoding/binary"
	"sync"
	"sync/atomic"

	"github.com/m-motawea/gSwitch/dataplane"
	"github.com/m-motawea/pipeline"
)

type ControlMessage struct {
	InFrame      *dataplane.IncomingFrame
	PreMessage   interface{} // To be able to reconstruct the packet again
	LayerPayload interface{} // To separate each leayer payload
	OutPorts     []*dataplane.SwitchPort
	ParentSwitch *Switch
	NextHop      string // IP address of the next hop (in case of routed traffic)
}

type ControlProcessFuncPair struct {
	InFunc  func(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage
	OutFunc func(proc pipeline.PipelineProcess, msg pipeline.PipelineMessage) pipeline.PipelineMessage
	Init    func(sw *Switch)
}

var ControlProcs map[int]map[string]ControlProcessFuncPair

var (
	MessageStore sync.Map
	messageIDSeq atomic.Uint64
)

func init() {
	ControlProcs = map[int]map[string]ControlProcessFuncPair{}
}

// StoreMessage saves the ControlMessage and returns an 8-byte ID for the Pipeline payload
func StoreMessage(msg ControlMessage) []byte {
	id := messageIDSeq.Add(1)
	MessageStore.Store(id, msg)
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, id)
	return b
}

// FetchMessage retrieves the original ControlMessage using the byte slice payload
func FetchMessage(payload []byte) (ControlMessage, bool) {
	if len(payload) < 8 {
		return ControlMessage{}, false
	}
	id := binary.LittleEndian.Uint64(payload)
	val, ok := MessageStore.Load(id)
	if !ok {
		return ControlMessage{}, false
	}
	return val.(ControlMessage), true
}

// DropMessage deletes the message mapping when fully consumed by the Dataplane natively
func DropMessage(payload []byte) (ControlMessage, bool) {
	if len(payload) < 8 {
		return ControlMessage{}, false
	}
	id := binary.LittleEndian.Uint64(payload)
	val, ok := MessageStore.LoadAndDelete(id)
	if !ok {
		return ControlMessage{}, false
	}
	return val.(ControlMessage), true
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
