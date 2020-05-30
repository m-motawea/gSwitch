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

2- Document Current Processes

3- Document Inter-Layer Communication (Adapters)
