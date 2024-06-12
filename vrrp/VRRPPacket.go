package vrrp

import (
	"errors"
	"fmt"
	"net"
	"unsafe"
)

type VRRPPacket struct {
	Header    [8]byte
	IPAddress [][4]byte
	Pshdr     *PseudoHeader
}

type PseudoHeader struct {
	Saddr    net.IP
	Daddr    net.IP
	Zero     uint8
	Protocol uint8
	Len      uint16
}

func (psh *PseudoHeader) ToBytes() []byte {
	var octets = make([]byte, 36)
	copy(octets, psh.Saddr)
	copy(octets[16:], psh.Daddr)
	copy(octets[32:], []byte{psh.Zero, psh.Protocol, byte(psh.Len >> 8), byte(psh.Len)})
	return octets
}

func FromBytes(IPvXVersion byte, octets []byte) (*VRRPPacket, error) {
	if len(octets) < 8 {
		return nil, errors.New("faulty VRRP packet size")
	}
	var packet VRRPPacket
	for index := 0; index < 8; index++ {
		packet.Header[index] = octets[index]
	}
	//todo validate the number of IPvX addresses
	var countofaddrs = int(packet.GetIPvXAddrCount())
	switch IPvXVersion {
	case 4:
	case 6:
		countofaddrs = countofaddrs * 4
	default:
		return nil, fmt.Errorf("faulty IPvX version %d", IPvXVersion)
	}
	//to compatible with VRRP v2 packet, ignore the auth info
	if 8+countofaddrs*4 > len(octets) {
		return nil, fmt.Errorf("The value of filed IPvXAddrCount doesn't match the length of octets")
	}
	for index := 0; index < countofaddrs; index++ {
		var addr [4]byte
		addr[0] = octets[8+4*index]
		addr[1] = octets[8+4*index+1]
		addr[2] = octets[8+4*index+2]
		addr[3] = octets[8+4*index+3]
		packet.IPAddress = append(packet.IPAddress, addr)
	}
	return &packet, nil
}

func (packet *VRRPPacket) GetIPvXAddr(version byte) (addrs []net.IP) {
	switch version {
	case 4:
		for index := range packet.IPAddress {
			addrs = append(addrs, net.IPv4(
				packet.IPAddress[index][0],
				packet.IPAddress[index][1],
				packet.IPAddress[index][2],
				packet.IPAddress[index][3]))
		}
		return addrs
	case 6:
		for index := 0; index < int(packet.GetIPvXAddrCount()); index++ {
			var p = make(net.IP, net.IPv6len)
			for i := 0; i < 4; i++ {
				copy(p[4*i:], packet.IPAddress[index*4+i][:])
			}
			addrs = append(addrs, p)
		}
		return addrs
	default:
		return nil
	}
}

func (packet *VRRPPacket) AddIPvXAddr(version byte, ip net.IP) {
	switch version {
	case 4:
		packet.IPAddress = append(packet.IPAddress, [4]byte{ip[12], ip[13], ip[14], ip[15]})
		packet.setIPvXAddrCount(packet.GetIPvXAddrCount() + 1)
		//todo byte maybe overflow
	case 6:
		for index := 0; index < 4; index++ {
			packet.IPAddress = append(packet.IPAddress, [4]byte{ip[index*4+0], ip[index*4+1], ip[index*4+2], ip[index*4+3]})
		}
		packet.setIPvXAddrCount(packet.GetIPvXAddrCount() + 1)
	default:
		panic("VRRPPacket.AddIPvXAddr: only support IPv4 and IPv6 address")
	}
}

func (packet *VRRPPacket) GetVersion() byte {
	return (packet.Header[0] & 240) >> 4
}

func (packet *VRRPPacket) SetVersion(Version VRRPVersion) {
	packet.Header[0] = (packet.Header[0] & 15) | (byte(Version) << 4)
}

func (packet *VRRPPacket) GetType() byte {
	return packet.Header[0] & 15
}

func (packet *VRRPPacket) SetType() {
	packet.Header[0] = (packet.Header[0] & 240) | 1
}

func (packet *VRRPPacket) GetVirtualRouterID() byte {
	return packet.Header[1]
}

func (packet *VRRPPacket) SetVirtualRouterID(VirtualRouterID byte) {
	packet.Header[1] = VirtualRouterID
}

func (packet *VRRPPacket) GetPriority() byte {
	return packet.Header[2]
}

func (packet *VRRPPacket) SetPriority(Priority byte) {
	packet.Header[2] = Priority
}

func (packet *VRRPPacket) GetIPvXAddrCount() byte {
	return packet.Header[3]
}

func (packet *VRRPPacket) setIPvXAddrCount(count byte) {
	packet.Header[3] = count
}

func (packet *VRRPPacket) GetAdvertisementInterval() uint16 {
	return uint16(packet.Header[4]&15)<<8 | uint16(packet.Header[5])
}

func (packet *VRRPPacket) SetAdvertisementInterval(interval uint16) {
	packet.Header[4] = (packet.Header[4] & 240) | byte((interval>>8)&15)
	packet.Header[5] = byte(interval)
}

func (packet *VRRPPacket) GetCheckSum() uint16 {
	return uint16(packet.Header[6])<<8 | uint16(packet.Header[7])
}

func (packet *VRRPPacket) SetCheckSum(pshdr *PseudoHeader) {
	var PointerAdd = func(ptr unsafe.Pointer, bytes int) unsafe.Pointer {
		return unsafe.Pointer(uintptr(ptr) + uintptr(bytes))
	}
	var octets = pshdr.ToBytes()
	octets = append(octets, packet.ToBytes()...)
	var x = len(octets)
	var ptr = unsafe.Pointer(&octets[0])
	var sum uint32
	for x > 1 {
		sum = sum + uint32(*(*uint16)(ptr))
		ptr = PointerAdd(ptr, 2)
		x = x - 2
	}
	if x > 0 {
		sum = sum + uint32(*((*uint8)(ptr)))
	}
	for (sum >> 16) > 0 {
		sum = sum&65535 + sum>>16
	}
	sum = ^sum
	packet.Header[7] = byte(sum >> 8)
	packet.Header[6] = byte(sum)
}

func (packet *VRRPPacket) ValidateCheckSum(pshdr *PseudoHeader) bool {
	var PointerAdd = func(ptr unsafe.Pointer, bytes int) unsafe.Pointer {
		return unsafe.Pointer(uintptr(ptr) + uintptr(bytes))
	}
	var octets = pshdr.ToBytes()
	octets = append(octets, packet.ToBytes()...)
	var x = len(octets)
	var ptr = unsafe.Pointer(&octets[0])
	var sum uint32
	for x > 1 {
		sum = sum + uint32(*(*uint16)(ptr))
		ptr = PointerAdd(ptr, 2)
		x = x - 2
	}
	if x > 0 {
		sum = sum + uint32(*((*uint8)(ptr)))
	}
	for (sum >> 16) > 0 {
		sum = sum&65535 + sum>>16
	}
	if uint16(sum) == 65535 {
		return true
	} else {
		return false
	}
}

func (packet *VRRPPacket) ToBytes() []byte {
	var payload = make([]byte, 8+len(packet.IPAddress)*4)
	copy(payload, packet.Header[:])
	for index := range packet.IPAddress {
		copy(payload[8+index*4:], packet.IPAddress[index][:])
	}
	return payload
}
