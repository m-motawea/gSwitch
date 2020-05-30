#!/bin/bash

set -e

ip netns add h1
ip netns add h2
ip netns add h3
ip netns add h4
ip netns add h5
ip netns add sw

ip link add h1 type veth peer name sw1
ip link add h2 type veth peer name sw2
ip link add h3 type veth peer name sw3
ip link add h4 type veth peer name sw4
# ip link add h5 type veth peer name sw5
# ip link add sw type veth peer name sw-mgmt # veth pair used both ends in sw namepace to reach running processes on this namespace

ip link set netns sw sw1
ip link set netns sw sw2
ip link set netns sw sw3
ip link set netns sw sw4
# ip link set netns sw sw5
# ip link set netns sw sw
# ip link set netns sw sw-mgmt

ip link set netns h1 h1
ip link set netns h2 h2
ip link set netns h3 h3
ip link set netns h4 h4
# ip link set netns h5 h5

ip -n h1 link set dev h1 up
ip -n h2 link set dev h2 up
ip -n h3 link set dev h3 up
ip -n h4 link set dev h4 up
# ip -n h5 link set dev h5 up

# h5-sw5 trunk
# ip -n h5 link add link h5 name h5.1 type vlan id 1
# ip -n h5 link add link h5 name h5.10 type vlan id 10
# ip -n sw link add link sw5 name sw5.1 type vlan id 1
# ip -n sw link add link sw5 name sw5.10 type vlan id 10


ip -n sw link set dev sw1 up
ip -n sw link set dev sw2 up
ip -n sw link set dev sw3 up
ip -n sw link set dev sw4 up
# ip -n sw link set dev sw5 up
# ip -n sw link set dev sw-mgmt up
# ip -n sw link set dev sw up
# ip -n sw link set dev sw5.1 up
# ip -n sw link set dev sw5.10 up
# ip -n h5 link set dev h5.1 up
# ip -n h5 link set dev h5.10 up

ip -n h1 addr add 10.1.1.10/24 dev h1
ip -n h2 addr add 10.1.1.20/24 dev h2
ip -n h3 addr add 10.10.1.30/24 dev h3
ip -n h4 addr add 10.10.1.40/24 dev h4
# ip -n h5 addr add 10.1.1.50/24 dev h5.1
# ip -n h5 addr add 10.10.1.50/24 dev h5.10
# ip -n sw addr add 10.1.1.254/24 dev sw

ip -n h1 route add 10.10.0.0/16 via 10.1.1.1
ip -n h2 route add 10.10.0.0/16 via 10.1.1.1
ip -n h3 route add 10.1.0.0/16 via 10.10.1.1
ip -n h4 route add 10.1.0.0/16 via 10.10.1.1

echo "test environment created successfully!"
echo "to start the switch use: ip netns exec sw ./gSwitch"