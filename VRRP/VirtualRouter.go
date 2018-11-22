package VRRP

import (
	"fmt"
	"net"
)

type VRRPStateMachine struct {
	IPvXVersion      byte
	IPAddr           net.IP
	State            int
	RespondingRouter *VirtualRouter
}

type VirtualRouter struct {
	VRID                          byte
	Priority                      byte
	AdvertisementInterval         uint16
	AdvertisementIntervalOfMaster uint16
	SkewTime                      uint16
	MasterDownInterval            uint16
	Preempt                       bool
	AcceptMode                    bool
	Owner                         bool
	VirtualRouterMACAddressIPv4   net.HardwareAddr
	VirtualRouterMACAddressIPv6   net.HardwareAddr
	BindedIPvXAddr                map[[16]byte]*VRRPStateMachine
}

func NewVirtualRouter() *VirtualRouter {
	return &VirtualRouter{BindedIPvXAddr: make(map[[16]byte]*VRRPStateMachine)}
}

func (r *VirtualRouter) SetVRID(ID byte) *VirtualRouter {
	r.VRID = ID
	r.VirtualRouterMACAddressIPv4, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-01-%X", ID))
	r.VirtualRouterMACAddressIPv6, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-02-%X", ID))
	return r
}

func (r *VirtualRouter) SetPriority(Priority byte) *VirtualRouter {
	if r.Owner == true {
		Priority = 255
	}
	r.Priority = Priority
	return r
}

func (r *VirtualRouter) SetAdvInterval(Interval uint16) *VirtualRouter {
	r.AdvertisementInterval = Interval
	return r
}

func (r *VirtualRouter) SetMasterAdvInterval(Interval uint16) *VirtualRouter {
	r.AdvertisementIntervalOfMaster = Interval
	r.SkewTime = r.AdvertisementIntervalOfMaster - r.AdvertisementIntervalOfMaster*uint16(r.Priority)/256
	r.MasterDownInterval = 3*r.AdvertisementIntervalOfMaster + r.SkewTime
	//从MasterDownInterval和SkewTime的计算方式来看，同一组VirtualRouter中，Priority越高的Router越快的认为某个Master失效
	return r
}

func (r *VirtualRouter) SetPreemptMode(flag bool) *VirtualRouter {
	r.Preempt = flag
	return r
}

func (r *VirtualRouter) SetAcceptMode(flag bool) *VirtualRouter {
	r.AcceptMode = flag
	return r
}

func (r *VirtualRouter) SetOwner(flag bool) *VirtualRouter {
	r.Owner = flag
	return r
}

func (r *VirtualRouter) AddIPvXAddr(version byte, ip net.IP) {
	var key [16]byte
	copy(key[:], ip)
	if _, ok := r.BindedIPvXAddr[key]; ok {
		//todo log this
	} else {
		var smachine = &VRRPStateMachine{
			IPvXVersion: version,
			IPAddr:      ip,
			State:       INIT,
		}
		r.BindedIPvXAddr[key] = smachine
	}
}

func (r *VirtualRouter) RemoveIPvXAddr(ip net.IP) error {
	var key [16]byte
	copy(key[:], ip)
	if _, ok := r.BindedIPvXAddr[key]; ok {
		delete(r.BindedIPvXAddr, key)
		return nil
	} else {
		return fmt.Errorf("error occurred when remove %v, unexist", ip)
	}
}
