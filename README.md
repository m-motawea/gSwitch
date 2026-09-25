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

- `Trunk`: whether the port is trunk or access port. Trunk ports carry 802.1Q tagged frames for the allowed VLANs; the first VLAN in `AllowedVLANs` is the native VLAN used for untagged frames

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
* this will create 5 namespaces as hosts (`h1`,..`h5`) and a one as switch `sw`
* `h1` & `h2` IP address are `10.1.1.10` and `10.1.1.20`
* `h3` & `h4` IP address are `10.10.1.30` and `10.10.1.40`
* `h5` is connected over a trunk and uses VLAN sub-interfaces: `h5.1` (`10.1.1.50`), `h5.10` (`10.10.1.50`) and `h5.20` (`10.20.1.50`, VLAN 20 is not allowed on the trunk)

4- Start the switch in the `sw` namepace with the default config in the package:
```bash
sudo ip netns exec sw ./gSwitch
```
* `h1` and `h2` are connected to `sw` as access ports on vlan 1
* `h3`and `h4`are connected to `sw` as access ports on vlan 10
* `h5` is connected to `sw5`, a trunk port allowing vlans 1 and 10
* Control processes include the `L2Switch`, `ARP`, `IPv4`, `ICMP` and `Routing` as well as each layer adapter process.

5- Test connectivity example:
```bash
sudo ip netns exec h1 ping 10.1.1.20 # connection to h2
sudo ip netns exec h1 ping 10.10.1.40 # connection to h4 (routed)
sudo ip netns exec h3 ping 10.10.1.50 # connection to h5.10 over the trunk
```

6- Clean the test environment:
```bash
sudo ./scripts/env_destroy.sh
```

### Trunk test (Linux, needs root and `tcpdump`):
```bash
go build
sudo ./scripts/trunk_test.sh
```
This builds the namespace environment, runs the switch with `config.toml`, sends traffic across the `h5`/`sw5` trunk and checks tcpdump captures: frames on the trunk carry the right 802.1Q tag, access ports stay untagged, VLANs stay isolated and the disallowed VLAN 20 is dropped. Captures are kept for inspection (set `OUT_DIR` to choose where).

The kernel strips VLAN tags on receive (`rx-vlan-offload`), so the dataplane reads them back from `PACKET_AUXDATA` and re-inserts them (see `dataplane/packetconn_linux.go`).

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
1- Handle L2 Loops Using `STP` (needs support for `802.3`)

2- Document Current Processes
