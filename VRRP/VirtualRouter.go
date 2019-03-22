package VRRP

import (
	"VRRP/logger"
	"fmt"
	"net"
	"time"
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
	ProtectedIPaddrs    map[[16]byte]bool
	State               int
	IPlayerInterface    IPConnection
	IPAddrAnnouncer     AddrAnnouncer
	EventChannel        chan EVENT
	PacketQueue         chan *VRRPPacket
	advertisementTicker *time.Ticker
	masterDownTimer     *time.Timer
	transitionHandler   map[transition]func() error
}

func NewVirtualRouter(VRID byte, nif string, Owner bool, IPvX byte) *VirtualRouter {
	if IPvX != IPv4 && IPvX != IPv6 {
		logger.GLoger.Printf(logger.FATAL, "NewVirtualRouter: parameter IPvx must be IPv4 or IPv6")
	}
	var vr = &VirtualRouter{ProtectedIPaddrs: make(map[[16]byte]bool)}
	vr.VRID = VRID
	vr.VirtualRouterMACAddressIPv4, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-01-%X", VRID))
	vr.VirtualRouterMACAddressIPv6, _ = net.ParseMAC(fmt.Sprintf("00-00-5E-00-02-%X", VRID))
	vr.Owner = Owner
	if Owner {
		vr.Priority = 255
	}
	vr.Priority = 100
	//set Initi
	vr.State = INIT

	//init event channel and packet queue
	vr.EventChannel = make(chan EVENT, EVENTCHANNELSIZE)
	vr.PacketQueue = make(chan *VRRPPacket, PACKETQUEUESIZE)

	vr.IPvX = IPvX
	var NetworkInterface, errOfGetIF = net.InterfaceByName(nif)
	if errOfGetIF != nil {
		logger.GLoger.Printf(logger.FATAL, "NewVirtualRouter: %v", errOfGetIF)
	}
	vr.NetInterface = NetworkInterface
	//find preferred local IP address
	if preferred, errOfGetPreferred := findIPbyInterface(NetworkInterface, IPvX); errOfGetPreferred != nil {
		logger.GLoger.Printf(logger.FATAL, "NewVirtualRouter: %v", errOfGetPreferred)
	} else {
		vr.preferredSourceIP = preferred
	}
	if IPvX == IPv4 {
		//set up ARP client
		vr.IPAddrAnnouncer = NewIPv4AddrAnnouncer(NetworkInterface)
		//set up IPv4 interface
		vr.IPlayerInterface = NewIPv4Conn(vr.preferredSourceIP, VRRPMultiAddrIPv4)
	} else {
		//set up ND client
		vr.IPAddrAnnouncer = NewIPIPv6AddrAnnouncer(NetworkInterface)
		//set up IPv6 interface
		vr.IPlayerInterface = NewIPv6Con(vr.preferredSourceIP, VRRPMultiAddrIPv6)
	}
	logger.GLoger.Printf(logger.INFO, "virtual router %v initialized, working on %v", VRID, nif)
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
		logger.GLoger.Printf(logger.ERROR, "VirtualRouter.AddIPvXAddr: add redundant IP addr %v", ip)
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
		logger.GLoger.Printf(logger.ERROR, "VirtualRouter.RemoveIPvXAddr: remove inexistent IP addr %v", ip)
	}
}

func (r *VirtualRouter) MasterDown() {
	logger.GLoger.Printf(logger.INFO, "master of virtual router %v unreachable", r.VRID)
}

func (r *VirtualRouter) SendAdvertMessage() {
	for k := range r.ProtectedIPaddrs {
		logger.GLoger.Printf(logger.DEBUG, "send advert message of IP %v", net.IP(k[:]))
	}
	//todo move var x = r.AssembleVRRPPacket() to upper level
	var x = r.AssembleVRRPPacket()
	if errOfWrite := r.IPlayerInterface.WriteMessage(x); errOfWrite != nil {
		logger.GLoger.Printf(logger.ERROR, "VirtualRouter.WriteMessage: %v", errOfWrite)
	}
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
	if r.IPvX == IPv4 {
		pshdr.Daddr = VRRPMultiAddrIPv4
	} else {
		pshdr.Daddr = VRRPMultiAddrIPv6
	}
	pshdr.Len = uint16(len(packet.ToBytes()))
	pshdr.Saddr = r.preferredSourceIP
	packet.SetCheckSum(&pshdr)
	return &packet
}

//FetchVRRPPacket read VRRP packet from IP layer then push into Packet queue
func (r *VirtualRouter) FetchVRRPPacket() {
	for {
		if packet, errofFetch := r.IPlayerInterface.ReadMessage(); errofFetch != nil {
			logger.GLoger.Printf(logger.ERROR, "VirtualRouter.FetchVRRPPacket: %v", errofFetch)
		} else {
			if r.VRID == packet.GetVirtualRouterID() {
				r.PacketQueue <- packet
			} else {
				logger.GLoger.Printf(logger.ERROR, "VirtualRouter.FetchVRRPPacket: received a advertisement with different ID")
			}

		}
		logger.GLoger.Printf(logger.DEBUG, "VirtualRouter.FetchVRRPPacket: received one advertisement")
	}
}

func (r *VirtualRouter) masterUP() {
	logger.GLoger.Printf(logger.INFO, "virtual router %v transition into MASTER state", r.VRID)
}

func (r *VirtualRouter) masterDown() {
	logger.GLoger.Printf(logger.INFO, "virtual router %v quit MASTER state", r.VRID)
}

func (r *VirtualRouter) makeAdvertTicker() {
	r.advertisementTicker = time.NewTicker(time.Duration(r.AdvertisementInterval*10) * time.Millisecond)
}

func (r *VirtualRouter) stopAdvertTicker() {
	r.advertisementTicker.Stop()
}

func (r *VirtualRouter) makeMasterDownTimer() {
	if r.masterDownTimer == nil {
		r.masterDownTimer = time.NewTimer(time.Duration(r.MasterDownInterval*10) * time.Millisecond)
	} else {
		r.resetMasterDownTimer()
	}
}

func (r *VirtualRouter) stopMasterDownTimer() {
	logger.GLoger.Printf(logger.DEBUG, "master down timer stopped")
	if !r.masterDownTimer.Stop() {
		select {
		case <-r.masterDownTimer.C:
		default:
		}
		logger.GLoger.Printf(logger.DEBUG, "master down timer expired before we stop it, drain the channel")
	}
}

func (r *VirtualRouter) resetMasterDownTimer() {
	r.stopMasterDownTimer()
	r.masterDownTimer.Reset(time.Duration(r.MasterDownInterval*10) * time.Millisecond)
}

func (r *VirtualRouter) resetMasterDownTimerToSkewTime() {
	r.stopMasterDownTimer()
	r.masterDownTimer.Reset(time.Duration(r.SkewTime*10) * time.Millisecond)
}

func (r *VirtualRouter) Enroll(transition2 transition, handler func() error) bool {
	if _, ok := r.transitionHandler[transition2]; ok {
		logger.GLoger.Printf(logger.INFO, fmt.Sprintf("VirtualRouter.Enroll(): handler of transition [%s] overwrited", transition2))
		r.transitionHandler[transition2] = handler
		return true
	}
	logger.GLoger.Printf(logger.INFO, fmt.Sprintf("VirtualRouter.Enroll(): handler of transition [%s] enrolled", transition2))
	r.transitionHandler[transition2] = handler
	return false
}

func (r *VirtualRouter) transitionDoWork(t transition) error {
	var work, ok = r.transitionHandler[t]
	if ok == false {
		return fmt.Errorf("VirtualRouter.transitionDoWork(): handler of [%s] does not exist", t)
	}
	var errOfwork error
	switch t {
	case Backup2Master:
		errOfwork = work()
	case Master2Backup:
		errOfwork = work()
	default:
		return fmt.Errorf("VirtualRouter.transitionDoWork(): unknown transition")
	}
	if errOfwork != nil {
		return fmt.Errorf("handler of transition [%s] failed, %s", t, errOfwork)
	}
	logger.GLoger.Printf(logger.INFO, fmt.Sprintf("handler of transition [%s] called", t))
	return nil
}

/////////////////////////////////////////
func largerThan(ip1, ip2 net.IP) bool {
	if len(ip1) != len(ip2) {
		logger.GLoger.Printf(logger.FATAL, "largerThan: two compared IP addresses must have the same length")
	}
	for index := range ip1 {
		if ip1[index] > ip2[index] {
			return true
		} else if ip1[index] < ip2[index] {
			return false
		}
	}
	return false
}

//EventLoop VRRP event loop to handle various triggered events
func (r *VirtualRouter) EventLoop() {
	switch r.State {
	case INIT:
		select {
		case event := <-r.EventChannel:
			if event == START {
				logger.GLoger.Printf(logger.INFO, "event %v received", event)
				if r.Priority == 255 || r.Owner {
					logger.GLoger.Printf(logger.INFO, "enter owner mode")
					r.SendAdvertMessage()
					if errOfarp := r.IPAddrAnnouncer.AnnounceAll(r); errOfarp != nil {
						logger.GLoger.Printf(logger.ERROR, "VirtualRouter.EventLoop: %v", errOfarp)
					}
					//set up advertisement timer
					r.makeAdvertTicker()
					r.masterUP()
					logger.GLoger.Printf(logger.DEBUG, "enter MASTER state")
					r.State = MASTER
				} else {
					logger.GLoger.Printf(logger.INFO, "VR is not the owner of protected IP addresses")
					r.SetMasterAdvInterval(r.AdvertisementInterval)
					//set up master down timer
					r.makeMasterDownTimer()
					logger.GLoger.Printf(logger.DEBUG, "enter BACKUP state")
					r.State = BACKUP
				}
			}
		}
	case MASTER:
		//check if shutdown event received
		select {
		case event := <-r.EventChannel:
			if event == SHUTDOWN {
				//close advert timer
				r.stopAdvertTicker()
				//send advertisement with priority 0
				var priority = r.Priority
				r.SetPriority(0)
				r.SendAdvertMessage()
				r.SetPriority(priority)
				//transition into INIT
				r.State = INIT
				logger.GLoger.Printf(logger.INFO, "event %v received", event)
				//maybe we can break out the event loop
			}
		case <-r.advertisementTicker.C: //check if advertisement timer fired
			r.SendAdvertMessage()
		default:
			//nothing to do, just break
		}
		//process incoming advertisement
		select {
		case packet := <-r.PacketQueue:
			if packet.GetPriority() == 0 {
				//I don't think we should anything here
			} else {
				if packet.GetPriority() > r.Priority || (packet.GetPriority() == r.Priority && largerThan(packet.Pshdr.Saddr, r.preferredSourceIP)) {
					//todo give up master role
					//cancel Advertisement timer
					r.stopAdvertTicker()
					//set up master down timer
					r.SetMasterAdvInterval(packet.GetAdvertisementInterval())
					r.makeMasterDownTimer()
					r.masterDown()
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
		case event := <-r.EventChannel:
			if event == SHUTDOWN {
				//close master down timer
				r.stopMasterDownTimer()
				//transition into INIT
				r.State = INIT
				logger.GLoger.Printf(logger.INFO, "event %s received", event)
			}
		default:
		}
		//process incoming advertisement
		select {
		case packet := <-r.PacketQueue:
			if packet.GetPriority() == 0 {
				logger.GLoger.Printf(logger.INFO, "received an advertisement with priority 0, transit into MASTER state", r.VRID)
				//Set the Master_Down_Timer to Skew_Time
				r.resetMasterDownTimerToSkewTime()
			} else {
				if r.Preempt == false || packet.GetPriority() > r.Priority || (packet.GetPriority() == r.Priority && largerThan(packet.Pshdr.Saddr, r.preferredSourceIP)) {
					//reset master down timer
					r.SetMasterAdvInterval(packet.GetAdvertisementInterval())
					r.resetMasterDownTimer()
				} else {
					//nothing to do, just discard this one
				}
			}
		default:
			//nothing to do
		}
		select {
		//Master_Down_Timer fired
		case <-r.masterDownTimer.C:
			// Send an ADVERTISEMENT
			r.SendAdvertMessage()
			if errOfARP := r.IPAddrAnnouncer.AnnounceAll(r); errOfARP != nil {
				logger.GLoger.Printf(logger.ERROR, "VirtualRouter.EventLoop: %v", errOfARP)
			}
			//Set the Advertisement Timer to Advertisement interval
			r.makeAdvertTicker()
			r.masterUP()
			r.State = MASTER
		default:
			//nothing to do
		}

	}
}
