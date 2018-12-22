package VRRP

import (
	"VRRP/logger"
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
	//VRRP specification delimiter
	NetInterface        string
	IPvX                byte
	preferredSourceIP   net.IP
	VRRPAdvertTimer     *AdvertTimer
	VRRPMasterDownTimer *MasterDownTimer
	ProtectedIPaddrs    map[[16]byte]bool
	StateMachine        VRRPStateMachine
	IPlayerInterface    NetWorkInterface
}

func NewVirtualRouter(VRID byte, nif string, Owner bool, IPvX byte) *VirtualRouter {
	if IPvX != IPv4 && IPvX != IPv6 {
		panic("IPvx must be IPv4 or IPv6")
	}
	var vr = &VirtualRouter{ProtectedIPaddrs: make(map[[16]byte]bool)}
	vr.VRID = VRID
	vr.VirtualRouterMACAddressIPv4, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-01-%X", VRID))
	vr.VirtualRouterMACAddressIPv6, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-02-%X", VRID))
	vr.NetInterface = nif
	vr.Owner = Owner
	if Owner {
		vr.Priority = 255
	}
	vr.IPvX = IPvX
	if IPvX == IPv4 {
		if IPlayer, errofopeninterface := NewIPv4Interface(nif); errofopeninterface != nil {
			logger.GLoger.Printf(logger.FATAL, "error occurred when creating IP layer interface, %v", errofopeninterface)
			panic(errofopeninterface)
		} else {
			//create IP layer interface
			vr.IPlayerInterface = IPlayer
			//determine source IP addr of VRRP packet
			var NetworkInterface, _ = net.InterfaceByName(nif)
			if addrs, errofgetaddrs := NetworkInterface.Addrs(); errofgetaddrs != nil {
				logger.GLoger.Printf(logger.FATAL, "error occurred when get ip addresses of %v", nif)
				panic(errofgetaddrs)
			} else {
				var preferred net.IP = nil
				for _, addr := range addrs {
					if addr, _, errofparsecidr := net.ParseCIDR(addr.String()); errofparsecidr != nil {
						panic(errofparsecidr)
					} else {
						if addr.IsGlobalUnicast() {
							if tmp := addr.To4(); tmp != nil {
								preferred = tmp
								break
							}
						}
					}
				}
				if preferred == nil {
					panic("error occurred when getting preferred source IP, can not find usable IP address on " + nif)
				}
				vr.preferredSourceIP = preferred
			}
		}
	}
	//todo IPv6 interface
	return vr

}

func (r *VirtualRouter) SetPriority(Priority byte) *VirtualRouter {
	if r.Owner {
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

func (r *VirtualRouter) AddIPvXAddr(ip net.IP) {
	var key [16]byte
	copy(key[:], ip)
	if _, ok := r.ProtectedIPaddrs[key]; ok {
		logger.GLoger.Printf(logger.ERROR, "add redundant IP addr %v", ip)
	} else {
		r.ProtectedIPaddrs[key] = true
	}
}

func (r *VirtualRouter) RemoveIPvXAddr(ip net.IP) {
	var key [16]byte
	copy(key[:], ip)
	if _, ok := r.ProtectedIPaddrs[key]; ok {
		delete(r.ProtectedIPaddrs, key)
		logger.GLoger.Printf(logger.INFO, "IP %v removed", ip)
	} else {
		logger.GLoger.Printf(logger.ERROR, "remove inexistent IP addr %v", ip)
	}
}

func (r *VirtualRouter) MasterDown() {
	logger.GLoger.Printf(logger.INFO, "master of virtual router %v down detected", r.VRID)
}

func (r *VirtualRouter) SendAdvertMessage() {
	logger.GLoger.Printf(logger.DEBUG, "send advert message of virtual router %v", r.VRID)
}

func (r *VirtualRouter) StartTimers() {
	go r.VRRPAdvertTimer.Run()
	go r.VRRPMasterDownTimer.Run()
}

func (r *VirtualRouter) AssembleVRRPPacket() *VRRPPacket {
	var packet VRRPPacket
	packet.SetPriority(r.Priority)
	packet.SetVersion(VRRPv3)
	packet.SetVirtualRouterID(r.VRID)
	packet.SetAdvertisementInterval(r.AdvertisementInterval)
	packet.SetType()
	for k := range r.ProtectedIPaddrs {
		packet.AddIPvXAddr(r.IPvX, net.IP(k[:]))
	}
	var pshdr PseudoHeader
	pshdr.Protocol = VRRPIPProtocolNumber
	pshdr.Daddr = VRRPMultiAddrIPv4
	pshdr.Len = uint16(len(packet.ToBytes()))
	pshdr.Saddr = r.preferredSourceIP
	packet.SetCheckSum(&pshdr)
	return &packet
}
