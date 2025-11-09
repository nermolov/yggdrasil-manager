#!/bin/bash

# starting point:
# https://github.com/yggdrasil-network/yggdrasil-go/blob/41f49faaa0b86b6b42420eb2bcc384dc3af0df51/misc/run-schannel-netns

# Creates a fully connected network of 4 nodes.
# All nodes are directly connected to each other.

# 1---2
# |\ /|
# | X |
# |/ \|
# 3---4

echo "-- Setting up network"

# Cleanup any existing namespaces
ip netns delete node1 2>/dev/null
ip netns delete node2 2>/dev/null
ip netns delete node3 2>/dev/null
ip netns delete node4 2>/dev/null

# Create network namespaces
ip netns add node1
ip netns add node2
ip netns add node3
ip netns add node4

# Create veth pairs for all connections
# Node 1 connections
ip link add veth12 type veth peer name veth21
ip link set veth12 netns node1 up
ip link set veth21 netns node2 up

ip link add veth13 type veth peer name veth31
ip link set veth13 netns node1 up
ip link set veth31 netns node3 up

ip link add veth14 type veth peer name veth41
ip link set veth14 netns node1 up
ip link set veth41 netns node4 up

# Node 2 connections (to 3 and 4, already connected to 1)
ip link add veth23 type veth peer name veth32
ip link set veth23 netns node2 up
ip link set veth32 netns node3 up

ip link add veth24 type veth peer name veth42
ip link set veth24 netns node2 up
ip link set veth42 netns node4 up

# Node 3 connection to 4 (already connected to 1 and 2)
ip link add veth34 type veth peer name veth43
ip link set veth34 netns node3 up
ip link set veth43 netns node4 up

# Enable loopback interfaces
ip netns exec node1 ip link set lo up
ip netns exec node2 ip link set lo up
ip netns exec node3 ip link set lo up
ip netns exec node4 ip link set lo up

echo "Set up complete, press Ctrl+C to clean up."

cleanup() {

  echo "-- Cleaning up network"
  ip netns delete node1
  ip netns delete node2
  ip netns delete node3
  ip netns delete node4
  exit 0
}

trap cleanup SIGINT

while true; do sleep 1; done
