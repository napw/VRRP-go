package VRRP

import "net"

type VRRPVersion byte

const (
	VRRPv1 VRRPVersion = 1
	VRRPv2 VRRPVersion = 2
	VRRPv3 VRRPVersion = 3
)

const (
	IPv4 = 4
	IPv6 = 6
)

const (
	INIT = iota
	MASTER
	BACKUP
)

const (
	VRRPMultiTTL         = 255
	VRRPIPProtocolNumber = 112
)

var VRRPMultiAddrIPv4 = net.IPv4(224, 0, 0, 18)

var BaordcastHADDR, _ = net.ParseMAC("ff:ff:ff:ff:ff:ff")

type EVENT byte

const (
	SHUTDOWN = iota
)

const PACKETQUEUESIZE = 1000
