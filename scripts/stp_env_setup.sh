#!/bin/bash

set -e

# create sw namespace to run gSwitch
ip netns add sw

# create ovs bridge with STP enabled
ovs-vsctl add-br br0
ovs-vsctl set bridge br0 stp_enable=true

# create hosts
ip netns add h1
ip netns add h2
ip netns add h3
ip netns add h4

# create host links
ip link add h1 type veth peer name sw1
ip link add h2 type veth peer name sw2
ip link add h3 type veth peer name sw3
ip link add h4 type veth peer name sw4

ip link set netns h1 h1
ip link set netns h2 h2
ip link set netns h3 h3
ip link set netns h4 h4

# connect hosts to gSwitch namespace
ip link set netns sw sw1
ip link set netns sw sw2

# connect hosts to ovs bridge
ovs-vsctl add-port br0 sw3
ovs-vsctl add-port br0 sw4

# create inter-switch links
ip link add gsw1 type veth peer name ovs-br1
ip link add gsw2 type veth peer name ovs-br2

# connect ovs to gSwitch namespace
ip link set netns sw gsw1
ip link set netns sw gsw2
ovs-vsctl add-port br0 ovs-br1
ovs-vsctl add-port br0 ovs-br2

# up host links
ip -n h1 link set dev h1 up
ip -n h2 link set dev h2 up
ip -n h3 link set dev h3 up
ip -n h4 link set dev h4 up
ip -n sw link set dev sw1 up
ip -n sw link set dev sw2 up
ip link set dev sw3 up
ip link set dev sw4 up

# up inter-switch links
ip -n sw link set dev gsw1 up
ip -n sw link set dev gsw2 up
ip link set dev ovs-br1 up
ip link set dev ovs-br2 up

# configure host addresses
ip -n h1 addr add 10.1.1.10/24 dev h1
ip -n h2 addr add 10.1.1.20/24 dev h2
ip -n h3 addr add 10.1.1.30/24 dev h3
ip -n h4 addr add 10.1.1.40/24 dev h4


echo "test environment created successfully!"
echo "to start the switch use: ip netns exec sw ./gSwitch"
