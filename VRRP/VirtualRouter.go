package VRRP

import (
	"fmt"
	"net"
)

type VRRPStateMachine struct {
	IPvXVersion      int
	IPAddr           net.IP
	state            int
	RespondingRouter *VirtualRouter
}

func NewVRRPSM(v int, ip net.IP, r *VirtualRouter) *VRRPStateMachine {
	return &VRRPStateMachine{IPvXVersion: v, IPAddr: ip, state: INIT, RespondingRouter: r}
}

func (m *VRRPStateMachine) State() int {
	return m.state
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
	VirtualRouterMACAddressIPv4   net.HardwareAddr
	VirtualRouterMACAddressIPv6   net.HardwareAddr
}

func (r *VirtualRouter) SetVRID(ID byte) *VirtualRouter {
	r.VRID = ID
	r.VirtualRouterMACAddressIPv4, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-01-%X", ID))
	r.VirtualRouterMACAddressIPv6, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-02-%X", ID))
	return r
}

func (r *VirtualRouter) SetPriority(Priority byte) *VirtualRouter {
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
