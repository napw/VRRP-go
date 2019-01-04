package VRRP

import "net"

type VRRPVersion byte

const (
	VRRPv1 VRRPVersion = 1
	VRRPv2 VRRPVersion = 2
	VRRPv3 VRRPVersion = 3
)

func (v *VRRPVersion) String() string {
	switch *v {
	case 1:
		return "VRRPVersion1"
	case 2:
		return "VRRPVersion2"
	case 3:
		return "VRRPVersion3"
	default:
		return "unknown VRRP version"
	}
}

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
