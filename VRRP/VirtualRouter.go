package VRRP

import (
	"VRRP/logger"
	"fmt"
	"net"
)

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
	NetInterface        *net.Interface
	IPvX                byte
	preferredSourceIP   net.IP
	VRRPAdvertTimer     *AdvertTimer
	VRRPMasterDownTimer *MasterDownTimer
	ProtectedIPaddrs    map[[16]byte]bool
	State               int
	IPlayerInterface    NetWorkInterface
	IPAddrAnnouncer     AddrAnnouncer
	EventChannel        chan EVENT
	PacketQueue         chan *VRRPPacket
}

func NewVirtualRouter(VRID byte, nif string, Owner bool, IPvX byte) *VirtualRouter {
	if IPvX != IPv4 && IPvX != IPv6 {
		panic("IPvx must be IPv4 or IPv6")
	}
	var vr = &VirtualRouter{ProtectedIPaddrs: make(map[[16]byte]bool)}
	vr.VRID = VRID
	vr.VirtualRouterMACAddressIPv4, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-01-%X", VRID))
	vr.VirtualRouterMACAddressIPv6, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-02-%X", VRID))

	vr.Owner = Owner
	if Owner {
		vr.Priority = 255
	}
	//set Initi
	vr.State = INIT

	//set up event channel and packet queue
	vr.EventChannel = make(chan EVENT)
	vr.PacketQueue = make(chan *VRRPPacket, PACKETQUEUESIZE)

	vr.IPvX = IPvX
	//set up IPv4 interface
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
				vr.NetInterface = NetworkInterface
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
				//set up ARP client
				vr.IPAddrAnnouncer = NewIPv4AddrAnnouncer(NetworkInterface)
			}
		}
	}
	//todo set up IPv6 interface
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
	return r
}

func (r *VirtualRouter) SetMasterAdvInterval(Interval uint16) *VirtualRouter {
	r.AdvertisementIntervalOfMaster = Interval
	r.SkewTime = r.AdvertisementIntervalOfMaster - uint16(float32(r.AdvertisementIntervalOfMaster)*float32(r.Priority)/256)
	r.MasterDownInterval = 3*r.AdvertisementIntervalOfMaster + r.SkewTime
	//从MasterDownInterval和SkewTime的计算方式来看，同一组VirtualRouter中，Priority越高的Router越快地认为某个Master失效
	return r
}

func (r *VirtualRouter) setMasterDownTimer() *VirtualRouter {
	r.VRRPMasterDownTimer = NewMasterDownTimer(int(r.MasterDownInterval), r.MasterDown)
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
	logger.GLoger.Printf(logger.INFO, "master of virtual router %v unreachable", r.VRID)
}

func (r *VirtualRouter) SendAdvertMessage() {
	for k := range r.ProtectedIPaddrs {
		logger.GLoger.Printf(logger.DEBUG, "send advert message of IP %v", net.IP(k[:]))
	}
	var x = r.AssembleVRRPPacket()
	r.IPlayerInterface.WriteMessage(x.ToBytes())
}

//AssembleVRRPPacket assemble VRRP advert packet
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

//FetchVRRPPacket read VRRP packet from IP layer then push into Packet queue
func (r *VirtualRouter) FetchVRRPPacket() {
	for {
		if octets, phdr, errofFetch := r.IPlayerInterface.ReadMessage(); errofFetch != nil {
			logger.GLoger.Printf(logger.ERROR, "error occurred when fetching advert packet, %v", errofFetch)
		} else {
			if packet, errofMakePacket := FromBytes(r.IPvX, octets); errofMakePacket != nil {
				logger.GLoger.Printf(logger.ERROR, "error occurred when unmarshalling advert packet, %v", errofMakePacket)
			} else {
				if !packet.ValidateCheckSum(phdr) {
					logger.GLoger.Printf(logger.ERROR, "received a illegal advert packet, pseudo header:{%v}", *phdr)
				} else {
					//maybe we need check if blocking here
					packet.Pshdr = phdr
					r.PacketQueue <- packet
				}
			}
		}
	}
}

func (r *VirtualRouter) masterUP() {
	logger.GLoger.Printf(logger.INFO, "virtual router %v transition into MASTER state", r.VRID)
}

func (r *VirtualRouter) masterDown() {
	logger.GLoger.Printf(logger.INFO, "virtual router %v quit MASTER state", r.VRID)
}

func (r *VirtualRouter) processIncomingAdvertPacket() {
	/////////////////////////////////////////
	var LargerThan = func(ip1, ip2 net.IP) bool {
		if len(ip1) != len(ip2) {
			panic("error occurred when comparing two IP addresses for advert packet, they should have the same length")
		}
		for index := range ip1 {
			if ip1[index] > ip2[index] {
				return true
			}
		}
		return false
	}
	/////////////////////////////////////////
	switch r.State {
	case MASTER:
		select {
		case packet := <-r.PacketQueue:
			if packet.GetPriority() == 0 {
				//I don't think we should anything here
			} else {
				if packet.GetPriority() > r.Priority || (packet.GetPriority() == r.Priority && LargerThan(packet.Pshdr.Saddr, r.preferredSourceIP)) {
					//todo give up master role
					r.VRRPAdvertTimer.Stop()
					r.SetMasterAdvInterval(packet.GetAdvertisementInterval())
					r.State = BACKUP
				} else {
					//just discard this one
				}
			}
		default:
			//nothing to do
		}
	case BACKUP:
		select {
		case packet := <-r.PacketQueue:
			if packet.GetPriority() == 0 {
				logger.GLoger.Printf(logger.INFO, "VRID:%v received one advertisement with priority 0, transition into MASTER state", r.VRID)
				//todo Set the Master_Down_Timer to Skew_Time
			} else {
				if r.Preempt == false || packet.GetPriority() > r.Priority {
					//todo reset master down timer
				} else {
					//nothing to do, just discard this one
				}
			}

		default:
			//nothing to do
		}
	}
}

func (r *VirtualRouter) EventLoop() {
	switch r.State {
	case INIT:
		if r.Priority == 255 || r.Owner {
			logger.GLoger.Printf(logger.INFO, "enter owner mode")
			r.SendAdvertMessage()
			r.IPAddrAnnouncer.AnnounceAll(r)
			//todo set up advertisement timer
			r.State = MASTER
		} else {
			logger.GLoger.Printf(logger.INFO, "VR is not a owner")
			r.SetMasterAdvInterval(r.AdvertisementInterval)
			//todo set up master down timer
			r.State = BACKUP
		}
	case MASTER:
		//set up for master mode
		r.masterUP()
		//wait for shutdown event, or process incoming advertisement
		select {
		case event := <-r.EventChannel:
			if event == SHUTDOWN {
				//close advert timer
				r.VRRPAdvertTimer.Stop()
				//send advertisement with priority 0
				var priority = r.Priority
				r.SetPriority(0)
				r.SendAdvertMessage()
				r.SetPriority(priority)
				//transition into INIT
				r.State = INIT
			}
		default:
			if true {
				r.processIncomingAdvertPacket()
			}

		}
	case BACKUP:
		select {
		case event := <-r.EventChannel:
			if event == SHUTDOWN {
				//close master down timer
				r.VRRPMasterDownTimer.Stop()
				//transition into INIT
				r.State = INIT
			}
		default:
			if true {
				r.processIncomingAdvertPacket()
			}

		}

	}
}
