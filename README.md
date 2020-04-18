# gSwitch

This is a user space switch (network pipeline processing) written in go for learning purposes (still work in progress).


### Configuration:
```toml
[Redis]
Address = "127.0.0.1"
Port = 6379
Password = ""
DB = 0
Prefix = ""

[SwitchPorts]
    [SwitchPorts.sw1]
    Trunk = false
    AllowedVLANs = [1]
    Up = true

    [SwitchPorts.sw2]
    Trunk = true
    AllowedVLANs = [1]
    Up = true

    [SwitchPorts.sw3]
    Trunk = true
    AllowedVLANs = [1]
    Up = true

    [SwitchPorts.sw4]
    Trunk = true
    AllowedVLANs = [1]
    Up = false


[[ControlProcess]]
Layer = 2
Name = "Hub"
```


#### 1- Redis (not used yet):
Redis is the backend datastore for this switch during runtime.


#### 2- SwitchPorts:
This represents the ports that will be added to the switch.

- `Trunk`: whether the port is trunk or access port (not implemented yet)

- `AllowedVLANs`: in case Trunk is false, specify only one vlan number, otherwise it includes the allowed vlans on the trunk (eg. `[10, 11, 12]`)

- `Up`: represents the initial status of the port whether it should be brought up on startup or not


#### 3- ControlProcess:
Control processes are what defines how the traffic is handled by the switch. currently only a `L2Hub` and `L2Switch` are implemented.

- `Layer`: represents the layer this process handles

- `Name`: name of the process


- Control processes define a pair of functions for in and out traffic processing which will be used to create a two way process in the switch pipeline.
- The Control process is a pipeline process as described here (https://github.com/m-motawea/pipeline)
- The content strucure of the PipelineMessage in the switch pipeline is defined in `github.com/m-motawea/l2_switch/controlplane`

```go
type ControlMessage struct {
	InFrame *dataplane.IncomingFrame
	OutPorts []*dataplane.SwitchPort
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

## TODO:
1- Fix Trunk Forwarding

2- Add ARP Control Process

3- Add Static Routing Process
