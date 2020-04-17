#!/bin/bash
set -e

ip netns del h1
ip netns del h2
ip netns del h3
ip netns del h4
ip netns del sw