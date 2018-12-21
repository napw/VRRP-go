package VRRP

import (
	"VRRP/logger"
	"VRRP/network"
	"fmt"
	"net"
)

type VRRPStateMachine struct {
	State int
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
	NetInterface                  net.Interface
	VRRPAdvertTimer               *AdvertTimer
	VRRPMasterDownTimer           *MasterDownTimer
	ProtectedIPaddrs              map[[16]byte]bool
	StateMachine                  VRRPStateMachine
	IPlayerInterface              *network.NetWorkInterface
}

func NewVirtualRouter(VRID byte, nif net.Interface, Owner bool) *VirtualRouter {
	var vr = &VirtualRouter{BindedIPvXAddr: make(map[[16]byte]*VRRPStateMachine)}
	vr.VRID = VRID
	vr.VirtualRouterMACAddressIPv4, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-01-%X", VRID))
	vr.VirtualRouterMACAddressIPv6, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-02-%X", VRID))
	vr.NetInterface = nif
	vr.Owner = Owner
	return vr

}

func (r *VirtualRouter) SetPriority(Priority byte) *VirtualRouter {
	if r.Owner == true {
		Priority = 255
		return r
	}
	r.Priority = Priority
	return r
}

func (r *VirtualRouter) SetAdvInterval(Interval uint16) *VirtualRouter {
	r.AdvertisementInterval = Interval
	r.VRRPAdvertTimer = NewAdvertTimer(int(r.AdvertisementInterval), r.SendAdvertMessage)
	return r
}

func (r *VirtualRouter) SetMasterAdvInterval(Interval uint16) *VirtualRouter {
	r.AdvertisementIntervalOfMaster = Interval
	r.SkewTime = r.AdvertisementIntervalOfMaster - uint16(float32(r.AdvertisementIntervalOfMaster)*float32(r.Priority)/256)
	r.MasterDownInterval = 3*r.AdvertisementIntervalOfMaster + r.SkewTime
	fmt.Println("MasterDownInterval", r.MasterDownInterval)
	r.VRRPMasterDownTimer = NewMasterDownTimer(int(r.MasterDownInterval), r.MasterDown)
	//从MasterDownInterval和SkewTime的计算方式来看，同一组VirtualRouter中，Priority越高的Router越快地认为某个Master失效
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

func (r *VirtualRouter) MasterDown() {
	logger.GLoger.Printf(logger.INFO, "master of virtual router %v down detected", r.VRID)
}

func (r *VirtualRouter) SendAdvertMessage() {
	//logger.GLoger.Printf(logger.DEBUG,"send advert message of virtual router %v",r.VRID)
}

func (r *VirtualRouter) StartTimers() {
	go r.VRRPAdvertTimer.Run()
	go r.VRRPMasterDownTimer.Run()
}
