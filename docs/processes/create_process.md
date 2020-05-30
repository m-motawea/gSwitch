## Extending by Creating a Control Process:
- The Control process is a pipeline process as described here (https://github.com/m-motawea/pipeline)
- Control processes define a pair of functions for in and out traffic processing which will be used to create a two way process in the switch pipeline.
- The content strucure of the PipelineMessage in the switch pipeline is defined in `github.com/m-motawea/l2_switch/controlplane`

```go
type ControlMessage struct {
	InFrame      *dataplane.IncomingFrame
	PreMessage   interface{} // To be able to reconstruct the packet again
	LayerPayload interface{} // To separate each leayer payload
	OutPorts     []*dataplane.SwitchPort
	ParentSwitch *Switch
}
```

- Control processes are registered in `proc.go` by importing them as:
```go
_ "github.com/m-motawea/gSwitch/l2"
```

- Control processes implement ```init()``` function in their respective files that registers their pair in the control plane
```go
func init() {
	HubProcFuncPair := controlplane.ControlProcessFuncPair{
	InFunc:  HubInProc,     // handles ingress traffic 
        OutFunc: HubOutProc,    // handles egress traffic
        Init:   HubInitFunc,    // initializes any requirements before the pipeline is started that takes (*controlplane.Switch) as parameter. can be nil 
	}

	controlplane.RegisterLayerProc(2, "Hub", HubProcFuncPair)
}
```

- Each control process has map type storage for their presistence requirements defined as:
```go
type ProcStor map[string]interface{}
```

- Control processes can access their stor using ```ParentSwitch``` in the control message as below:
```go
msgContent, _ := msg.Content.(controlplane.ControlMessage)
stor := msgContent.ParentSwitch.Stor.GetStor(2, "Hub")
val := stor["number"]
stor["number"] = val.(int) + 1
log.Printf("\n\nHub Stor: %v \n\n", stor)
```