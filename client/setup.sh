#!/bin/sh

ip link set cw2 up
ip addr add 10.0.5.2/32 dev cw2
ip link set dev cw2 mtu 1420
ip route replace 10.0.5.0/24 dev cw2
