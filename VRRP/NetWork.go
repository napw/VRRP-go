package VRRP

import (
	"VRRP/logger"
	"fmt"
	"github.com/mdlayher/arp"
	"github.com/mdlayher/ndp"
	"net"

	"syscall"
	"time"
)

type IPConnection interface {
	WriteMessage(*VRRPPacket) error
	ReadMessage() (*VRRPPacket, error)
}

type AddrAnnouncer interface {
	AnnounceAll(vr *VirtualRouter) error
}

type IPv4AddrAnnouncer struct {
	ARPClient *arp.Client
}

type IPv6AddrAnnouncer struct {
	con *ndp.Conn
}

func NewIPIPv6AddrAnnouncer(nif *net.Interface) *IPv6AddrAnnouncer {
	var con, ip, errOfMakeNDPCon = ndp.Dial(nif, ndp.LinkLocal)
	if errOfMakeNDPCon != nil {
		logger.GLoger.Printf(logger.FATAL, "NewIPIPv6AddrAnnouncer: %v", errOfMakeNDPCon)
	}
	logger.GLoger.Printf(logger.INFO, "NDP client initialized, working on %v, source IP %v", nif.Name, ip)
	return &IPv6AddrAnnouncer{con: con}
}

func (nd *IPv6AddrAnnouncer) AnnounceAll(vr *VirtualRouter) error {
	for key := range vr.ProtectedIPaddrs {
		var multicastgroup, errOfParseMulticastGroup = ndp.SolicitedNodeMulticast(net.IP(key[:]))
		if errOfParseMulticastGroup != nil {
			logger.GLoger.Printf(logger.ERROR, "IPv6AddrAnnouncer.AnnounceAll: %v", errOfParseMulticastGroup)
		} else {
			//send unsolicited NeighborAdvertisement to refresh link layer address cache
			var msg = &ndp.NeighborAdvertisement{
				Override:      true,
				TargetAddress: net.IP(key[:]),
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      vr.NetInterface.HardwareAddr,
					},
				},
			}
			if errOfWrite := nd.con.WriteTo(msg, nil, multicastgroup); errOfWrite != nil {
				logger.GLoger.Printf(logger.ERROR, "IPv6AddrAnnouncer.AnnounceAll: %v", errOfWrite)
			} else {
				logger.GLoger.Printf(logger.INFO, "send unsolicited neighbor advertisement for %v", net.IP(key[:]))
			}
		}

	}

	return nil
}

//makeGratuitousPacket make gratuitous ARP packet with out payload
func (ar *IPv4AddrAnnouncer) makeGratuitousPacket() *arp.Packet {
	var packet arp.Packet
	packet.HardwareType = 1      //ethernet10m
	packet.ProtocolType = 0x0800 //IPv4
	packet.HardwareAddrLength = 6
	packet.IPLength = 4
	packet.Operation = 2 //response
	return &packet
}

//AnnounceAll send gratuitous ARP response for all protected IPv4 addresses
func (ar *IPv4AddrAnnouncer) AnnounceAll(vr *VirtualRouter) error {
	//todo add time limit
	if errofSetDealLine := ar.ARPClient.SetWriteDeadline(time.Now().Add(500 * time.Microsecond)); errofSetDealLine != nil {
		return fmt.Errorf("IPv4AddrAnnouncer.AnnounceAll: %v", errofSetDealLine)
	}
	var packet = ar.makeGratuitousPacket()
	for k := range vr.ProtectedIPaddrs {
		packet.SenderHardwareAddr = vr.NetInterface.HardwareAddr
		packet.SenderIP = net.IP(k[:]).To4()
		packet.TargetHardwareAddr = BaordcastHADDR
		packet.TargetIP = net.IP(k[:]).To4()
		logger.GLoger.Printf(logger.INFO, "send gratuitous arp for %v", net.IP(k[:]))
		if errofsendarp := ar.ARPClient.WriteTo(packet, BaordcastHADDR); errofsendarp != nil {
			return fmt.Errorf("IPv4AddrAnnouncer.AnnounceAll: %v", errofsendarp)
		}
	}
	return nil
}

func NewIPv4AddrAnnouncer(nif *net.Interface) *IPv4AddrAnnouncer {
	if aar, errofDialARP := arp.Dial(nif); errofDialARP != nil {
		panic(errofDialARP)
	} else {
		logger.GLoger.Printf(logger.DEBUG, "IPv4 addresses announcer created")
		return &IPv4AddrAnnouncer{ARPClient: aar}
	}

}

type IPv4Con struct {
	buffer     []byte
	remote     net.IP
	local      net.IP
	SendCon    *net.IPConn
	ReceiveCon *net.IPConn
}

type IPv6Con struct {
	buffer []byte
	oob    []byte
	remote net.IP
	local  net.IP
	Con    *net.IPConn
}

func ipConnection(local, remote net.IP) (*net.IPConn, error) {

	var conn *net.IPConn
	var errOfListenIP error
	//a little redundant
	//todo simplify here
	if local.IsLinkLocalUnicast() {
		var itf, errOfFind = findInterfacebyIP(local)
		if errOfFind != nil {
			return nil, fmt.Errorf("ipConnection: can't find zone info of %v", local)
		}
		conn, errOfListenIP = net.ListenIP("ip:112", &net.IPAddr{IP: local, Zone: itf.Name})
	} else {
		conn, errOfListenIP = net.ListenIP("ip:112", &net.IPAddr{IP: local})
	}
	if errOfListenIP != nil {
		return nil, errOfListenIP
	}
	var fd, errOfGetFD = conn.File()
	if errOfGetFD != nil {
		return nil, errOfGetFD
	}
	defer fd.Close()
	if remote.To4() != nil {
		//IPv4 mode
		//set hop limit
		if errOfSetHopLimit := syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IP, syscall.IP_MULTICAST_TTL, VRRPMultiTTL); errOfSetHopLimit != nil {
			return nil, fmt.Errorf("ipConnection: %v", errOfSetHopLimit)
		}
		//set tos
		if errOfSetTOS := syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IP, syscall.IP_TOS, 7); errOfSetTOS != nil {
			return nil, fmt.Errorf("ipConnection: %v", errOfSetTOS)
		}
		//disable multicast loop
		if errOfSetLoop := syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IP, syscall.IP_MULTICAST_LOOP, 0); errOfSetLoop != nil {
			return nil, fmt.Errorf("ipConnection: %v", errOfSetLoop)
		}
	} else {
		//IPv6 mode
		//set hop limit
		if errOfSetHOPLimit := syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IPV6, syscall.IPV6_MULTICAST_HOPS, 255); errOfSetHOPLimit != nil {
			return nil, fmt.Errorf("ipConnection: %v", errOfSetHOPLimit)
		}
		//disable multicast loop
		if errOfSetLoop := syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IPV6, syscall.IPV6_MULTICAST_LOOP, 0); errOfSetLoop != nil {
			return nil, fmt.Errorf("ipConnection: %v", errOfSetLoop)
		}
		//to receive the hop limit and dst address in oob
		if err := syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IPV6, syscall.IPV6_2292HOPLIMIT, 1); err != nil {
			return nil, fmt.Errorf("ipConnection: %v", err)
		}
		if err := syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IPV6, syscall.IPV6_2292PKTINFO, 1); err != nil {
			return nil, fmt.Errorf("ipConnection: %v", err)
		}

	}
	logger.GLoger.Printf(logger.INFO, "IP virtual connection established %v ==> %v", local, remote)
	return conn, nil
}

func makeMulticastIPv4Conn(multi, local net.IP) (*net.IPConn, error) {
	var conn, errOfListenIP = net.ListenIP("ip4:112", &net.IPAddr{IP: multi})
	if errOfListenIP != nil {
		return nil, fmt.Errorf("makeMulticastIPv4Conn: %v", errOfListenIP)
	}
	var fd, errOfGetFD = conn.File()
	if errOfGetFD != nil {
		return nil, fmt.Errorf("makeMulticastIPv4Conn: %v", errOfGetFD)
	}
	defer fd.Close()
	multi = multi.To4()
	local = local.To4()
	var mreq = &syscall.IPMreq{
		Multiaddr: [4]byte{multi[0], multi[1], multi[2], multi[3]},
		Interface: [4]byte{local[0], local[1], local[2], local[3]},
	}
	if errSetMreq := syscall.SetsockoptIPMreq(int(fd.Fd()), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, mreq); errSetMreq != nil {
		return nil, fmt.Errorf("makeMulticastIPv4Conn: %v", errSetMreq)
	}
	return conn, nil
}

func joinIPv6MulticastGroup(con *net.IPConn, local, remote net.IP) error {
	var fd, errOfGetFD = con.File()
	if errOfGetFD != nil {
		return fmt.Errorf("joinIPv6MulticastGroup: %v", errOfGetFD)
	}
	defer fd.Close()
	var mreq = &syscall.IPv6Mreq{}
	copy(mreq.Multiaddr[:], remote.To16())
	var IF, errOfGetIF = findInterfacebyIP(local)
	if errOfGetIF != nil {
		return fmt.Errorf("joinIPv6MulticastGroup: %v", errOfGetIF)
	}
	mreq.Interface = uint32(IF.Index)
	if errOfSetMreq := syscall.SetsockoptIPv6Mreq(int(fd.Fd()), syscall.IPPROTO_IPV6, syscall.IPV6_JOIN_GROUP, mreq); errOfSetMreq != nil {
		return fmt.Errorf("joinIPv6MulticastGroup: %v", errOfSetMreq)
	}
	logger.GLoger.Printf(logger.INFO, "Join IPv6 multicast group %v on %v", remote, IF.Name)
	return nil
}

func NewIPv4Conn(local, remote net.IP) IPConnection {
	var SendConn, errOfMakeIPConn = ipConnection(local, remote)
	if errOfMakeIPConn != nil {
		panic(errOfMakeIPConn)
	}
	var receiveConn, errOfMakeRecv = makeMulticastIPv4Conn(VRRPMultiAddrIPv4, local)
	if errOfMakeRecv != nil {
		panic(errOfMakeRecv)
	}
	return &IPv4Con{
		buffer:     make([]byte, 2048),
		local:      local,
		remote:     remote,
		SendCon:    SendConn,
		ReceiveCon: receiveConn,
	}

}

func (conn *IPv4Con) WriteMessage(packet *VRRPPacket) error {
	if _, err := conn.SendCon.WriteTo(packet.ToBytes(), &net.IPAddr{IP: conn.remote}); err != nil {
		return fmt.Errorf("IPv4Con.WriteMessage: %v", err)
	}
	return nil
}

func (conn *IPv4Con) ReadMessage() (*VRRPPacket, error) {
	var n, errOfRead = conn.ReceiveCon.Read(conn.buffer)
	if errOfRead != nil {
		return nil, fmt.Errorf("IPv4Con.ReadMessage: %v", errOfRead)
	}
	if n < 20 {
		return nil, fmt.Errorf("IPv4Con.ReadMessage: IP datagram lenght %v too small", n)
	}
	var hdrlen = (int(conn.buffer[0]) & 0x0f) << 2
	if hdrlen > n {
		return nil, fmt.Errorf("IPv4Con.ReadMessage: the header length %v is lagger than total length %V", hdrlen, n)
	}
	if conn.buffer[8] != 255 {
		return nil, fmt.Errorf("IPv4Con.ReadMessage: the TTL of IP datagram carring VRRP advertisment must equal to 255")
	}
	if advertisement, errOfUnmarshal := FromBytes(IPv4, conn.buffer[hdrlen:n]); errOfUnmarshal != nil {
		return nil, fmt.Errorf("IPv4Con.ReadMessage: %v", errOfUnmarshal)
	} else {
		if VRRPVersion(advertisement.GetVersion()) != VRRPv3 {
			return nil, fmt.Errorf("IPv4Con.ReadMessage: received an advertisement with %s", VRRPVersion(advertisement.GetVersion()))
		}
		var pshdr PseudoHeader
		pshdr.Saddr = net.IPv4(conn.buffer[12], conn.buffer[13], conn.buffer[14], conn.buffer[15]).To16()
		pshdr.Daddr = net.IPv4(conn.buffer[16], conn.buffer[17], conn.buffer[18], conn.buffer[19]).To16()
		pshdr.Protocol = VRRPIPProtocolNumber
		pshdr.Len = uint16(n - hdrlen)
		if !advertisement.ValidateCheckSum(&pshdr) {
			return nil, fmt.Errorf("IPv4Con.ReadMessage: validate the check sum of advertisement failed")
		} else {
			advertisement.Pshdr = &pshdr
			return advertisement, nil
		}
	}
}

func NewIPv6Con(local, remote net.IP) *IPv6Con {
	var con, errOfNewIPv6Con = ipConnection(local, remote)
	if errOfNewIPv6Con != nil {
		panic(fmt.Errorf("NewIPv6Con: %v", errOfNewIPv6Con))
	}
	if errOfJoinMG := joinIPv6MulticastGroup(con, local, remote); errOfJoinMG != nil {
		panic(fmt.Errorf("NewIPv6Con: %v", errOfJoinMG))
	}
	return &IPv6Con{
		buffer: make([]byte, 4096),
		oob:    make([]byte, 4096),
		local:  local,
		remote: remote,
		Con:    con,
	}
}

func (con *IPv6Con) WriteMessage(packet *VRRPPacket) error {
	if _, errOfWrite := con.Con.WriteToIP(packet.ToBytes(), &net.IPAddr{IP: con.remote}); errOfWrite != nil {
		return fmt.Errorf("IPv6Con.WriteMessage: %v", errOfWrite)
	}
	return nil
}

func (con *IPv6Con) ReadMessage() (*VRRPPacket, error) {
	var buffern, oobn, _, raddr, errOfRead = con.Con.ReadMsgIP(con.buffer, con.oob)
	if errOfRead != nil {
		return nil, fmt.Errorf("IPv6Con.ReadMessage: %v", errOfRead)
	}
	var oobdata, errOfParseOOB = syscall.ParseSocketControlMessage(con.oob[:oobn])
	if errOfParseOOB != nil {
		return nil, fmt.Errorf("IPv6Con.ReadMessage: %v", errOfParseOOB)
	}
	var (
		dst    net.IP
		TTL    byte
		GetTTL = false
	)
	for index := range oobdata {
		if oobdata[index].Header.Level != syscall.IPPROTO_IPV6 {
			continue
		}
		switch oobdata[index].Header.Type {
		case syscall.IPV6_2292HOPLIMIT:
			if len(oobdata[index].Data) == 0 {
				return nil, fmt.Errorf("IPv6Con.ReadMessage: invalid HOPLIMIT")
			}
			TTL = oobdata[index].Data[0]
			GetTTL = true
		case syscall.IPV6_2292PKTINFO:
			if len(oobdata[index].Data) < 16 {
				return nil, fmt.Errorf("IPv6Con.ReadMessage: invalid destination IP addrress length")
			}
			dst = net.IP(oobdata[index].Data[:16])
		}
	}
	if GetTTL == false {
		return nil, fmt.Errorf("IPv6Con.ReadMessage: HOPLIMIT not found")
	}
	if dst == nil {
		return nil, fmt.Errorf("IPv6Con.ReadMessage: destination address not found")
	}
	var pshdr = PseudoHeader{
		Daddr:    dst,
		Saddr:    raddr.IP,
		Protocol: VRRPIPProtocolNumber,
		Len:      uint16(buffern),
	}
	var advertisement, errOfUnmarshal = FromBytes(IPv6, con.buffer)
	if errOfUnmarshal != nil {
		return nil, fmt.Errorf("IPv6Con.ReadMessage: %v", errOfUnmarshal)
	}
	if TTL != 255 {
		return nil, fmt.Errorf("IPv6Con.ReadMessage: invalid HOPLIMIT")
	}
	if VRRPVersion(advertisement.GetVersion()) != VRRPv3 {
		return nil, fmt.Errorf("IPv6Con.ReadMessage: invalid VRRP version %v", advertisement.GetVersion())
	}
	if !advertisement.ValidateCheckSum(&pshdr) {
		return nil, fmt.Errorf("IPv6Con.ReadMessage: invalid check sum")
	}
	advertisement.Pshdr = &pshdr
	return advertisement, nil
}

func findIPbyInterface(itf *net.Interface, IPvX byte) (net.IP, error) {
	var addrs, errOfListAddrs = itf.Addrs()
	if errOfListAddrs != nil {
		return nil, fmt.Errorf("findIPbyInterface: %v", errOfListAddrs)
	}
	for index := range addrs {
		var ipaddr, _, errOfParseIP = net.ParseCIDR(addrs[index].String())
		if errOfParseIP != nil {
			return nil, fmt.Errorf("findIPbyInterface: %v", errOfParseIP)
		}
		if IPvX == IPv4 {
			if ipaddr.To4() != nil {
				if ipaddr.IsGlobalUnicast() {
					return ipaddr, nil
				}
			}
		} else {
			if ipaddr.To4() == nil {
				if ipaddr.IsLinkLocalUnicast() {
					return ipaddr, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("findIPbyInterface: can not find valid IP addrs on %v", itf.Name)
}

func findInterfacebyIP(ip net.IP) (*net.Interface, error) {
	if itfs, errOfListInterface := net.Interfaces(); errOfListInterface != nil {
		return nil, fmt.Errorf("findInterfacebyIP: %v", errOfListInterface)
	} else {
		for index := range itfs {
			if addrs, errOfListAddrs := itfs[index].Addrs(); errOfListAddrs != nil {
				return nil, fmt.Errorf("findInterfacebyIP: %v", errOfListAddrs)
			} else {
				for index1 := range addrs {
					var ipaddr, _, errOfParseIP = net.ParseCIDR(addrs[index1].String())
					if errOfParseIP != nil {
						return nil, fmt.Errorf("findInterfacebyIP: %v", errOfParseIP)
					}
					if ipaddr.Equal(ip) {
						return &itfs[index], nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("findInterfacebyIP: can't find the corresponding interface of %v", ip)
}
