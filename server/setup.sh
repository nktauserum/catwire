#!/bin/sh

ip link set dev cw0 up
ip addr add 10.0.5.1/24 dev cw0
ip link set dev cw0 mtu 1420
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
iptables -A FORWARD -i cw0 -j ACCEPT
iptables -A FORWARD -o cw0 -j ACCEPT
sysctl -w net.ipv4.ip_forward=1
