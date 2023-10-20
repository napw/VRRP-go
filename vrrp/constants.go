package vrrp

import (
	"net"
	"time"
)

type VRRPVersion byte

const (
	VRRPv1 VRRPVersion = 1
	VRRPv2 VRRPVersion = 2
	VRRPv3 VRRPVersion = 3
)

func (v VRRPVersion) String() string {
	switch v {
	case VRRPv1:
		return "VRRPVersion1"
	case VRRPv2:
		return "VRRPVersion2"
	case VRRPv3:
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
var VRRPMultiAddrIPv6 = net.ParseIP("FF02:0:0:0:0:0:0:12")

var BaordcastHADDR, _ = net.ParseMAC("ff:ff:ff:ff:ff:ff")

type EVENT byte

const (
	SHUTDOWN EVENT = iota
	START
)

func (e EVENT) String() string {
	switch e {
	case START:
		return "START"
	case SHUTDOWN:
		return "SHUTDOWN"
	default:
		return "unknown event"
	}
}

const PACKETQUEUESIZE = 1000
const EVENTCHANNELSIZE = 1

type transition int

func (t transition) String() string {
	switch t {
	case Master2Backup:
		return "master to backup"
	case Backup2Master:
		return "backup to master"
	case Init2Master:
		return "init to master"
	case Init2Backup:
		return "init to backup"
	case Backup2Init:
		return "backup to init"
	case Master2Init:
		return "master to init"
	default:
		return "unknown transition"
	}
}

const (
	Master2Backup transition = iota
	Backup2Master
	Init2Master
	Init2Backup
	Master2Init
	Backup2Init
)

var (
	defaultPreempt                    = true
	defaultPriority              byte = 100
	defaultAdvertisementInterval      = 1 * time.Second
)
