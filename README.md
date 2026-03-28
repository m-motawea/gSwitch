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
    AllowedVLANs = [1, 10]
    Up = true


[[ControlProcess]]
Layer = 2
Name = "L2Switch"

[[ControlProcess]]
Layer = 2
Name = "ARP"
ConfigFile = "etc/l2/ARPConfig.toml"

[[ControlProcess]]
Layer = 2
Name = "L2Adapter"
ConfigFile = "etc/l2/L2Adapter.toml"

[[ControlProcess]]
Layer = 3
Name = "IPv4"

[[ControlProcess]]
Layer = 3
Name = "Routing"
ConfigFile = "etc/l3/RoutingTable.toml"

[[ControlProcess]]
Layer = 3
Name = "ICMP"
ConfigFile = "etc/l3/ICMPConfig.toml"
```


#### 1- Redis (not used yet):
Redis is the backend datastore for this switch during runtime.


#### 2- SwitchPorts:
This represents the ports that will be added to the switch.

- `Trunk`: whether the port is trunk or access port (not implemented yet)

- `AllowedVLANs`: in case Trunk is false, specify only one vlan number, otherwise it includes the allowed vlans on the trunk (eg. `[10, 11, 12]`)

- `Up`: represents the initial status of the port whether it should be brought up on startup or not


#### 3- ControlProcess:
Control processes are what defines how the traffic is handled by the switch. The switch is built heavily around the distributed asynchronous pipeline `github.com/m-motawea/pipeline`.

With the recent shift to the distributed pipeline branch, Goroutine channel bottlenecks were alleviated with the `localqueue` messaging drivers over 8-byte IDs (`MessageStore` centralized registries) rather than pointer traversal.

Currently `L2Switch`, `L2Adapter`, `ARP`, `IPv4`, `Routing`, and `ICMP` processes are beautifully orchestrated and natively interact with one another bidirectionally. Processes are executed iteratively depending on `Layer` and `Name` order during Ingress and Reverse order during Egress.

- `Layer`: represents the layer this process handles

- `Name`: name of the process

- `ConfigFile`: path to process configuration file (if needed)


## Try It:
1- Get the Package
```bash
go get github.com/m-motawea/gSwitch
```

2- Go the package dir and build it
```bash
cd ~/go/src/github.com/m-motawea/gSwitch # change the location if you installed go in a custom location
go build
```

3- Initialize a test environment by network namespaces:
```bash
sudo ./scripts/env_setup.sh
```
* this will create 5 namespaces as hosts (`h1`,..`h4`) and a one as switch `sw`
* `h1` & `h2` IP address are `10.1.1.10` and `10.1.1.20`
* `h2` & `h3` IP address are `10.10.1.30` and `10.10.1.40`

4- Start the switch in the `sw` namepace with the default config in the package:
```bash
sudo ip netns exec sw ./gSwitch
```
* `h1` and `h2` are connected to `sw` as access ports on vlan 1
* `h3`and `h4`are connected to `sw` as access ports on vlan 10
* Control processes include the `L2Switch`, `ARP`, `IPv4`, `ICMP` and `Routing` as well as each layer adapter process.

5- Test connectivity example:
```bash
sudo ip netns exec h1 ping 10.1.1.20 # connection to h2
sudo ip netns exec h1 ping 10.10.1.40 # connection to h4 (routed)
```

6- Clean the test environment:
```bash
sudo ./scripts/env_destroy.sh
```

### Running unit and integration tests locally:

Before executing full integration tests verification, you must be successfully running the `distributed_pipeline` branch of the `github.com/m-motawea/pipeline` framework. This architecture replaces blocking concurrent Go channels with the `localqueue` driver and ID-based message map `StoreMessage()` pointer bypassing logic.

```bash
go mod edit -dropreplace github.com/m-motawea/pipeline
go get github.com/m-motawea/pipeline@distributed_pipeline
```

Run the robust E2E full-stack verification test natively:
```bash
go test -v ./controlplane -run TestSwitchEndToEnd_ICMP
```
This comprehensive unit test mimics a complete end-to-end framework, sending ICMP Echo Requests resolving dynamically back and forth using physical L2/L3 encapsulation layers and intercepting active dynamic hardware ARP negotiation requests automatically whenever a mapped address table drops.


## TODO:
1- Try to Fix Trunk Ports (due to stripped vlan tags)
* currently trunk link is not working but to get around this you can use subinterfaces.
* create a sub interface for each vlan and use the sub interface in configuration instaed of the Master.
```bash
ip link add link <master> name <sub name> type vlan id <id>
```
* make sure to set the trunk option for the subinterface as `false` other wise there will be two layers of 802.1Q.
```toml
[SwitchPorts."sw5.10"]
Trunk = false
AllowedVLANs = [10]
Up = true

[SwitchPorts."sw5.1"]
Trunk = false
AllowedVLANs = [1]
Up = true
```

2- Handle L2 Loops Using `STP` (needs support for `802.3`)

3- Document Current Processes
