package VRRP

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
