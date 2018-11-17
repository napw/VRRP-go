package network

import (
	"fmt"
	"golang.org/x/net/ipv4"
	"net"
)

type NetWorkInterface interface {
	WriteMessage([]byte) error
	ReadMessage() ([]byte, error)
	Close()
}

type IPv4Interface struct {
	buffer    []byte
	InterFace *net.Interface
	con       net.PacketConn
	rawConn   *ipv4.RawConn
}

func (iface *IPv4Interface) Close() {
	var VRRPAddr = net.IPAddr{IP: net.ParseIP(VRRPMultiAddr)}
	if errOfLeave := iface.rawConn.LeaveGroup(iface.InterFace, &VRRPAddr); errOfLeave != nil {
		//todo
	}
	iface.rawConn.Close()
	iface.con.Close()
}

func (iface *IPv4Interface) WriteMessage(payload []byte) error {
	var ipheader = &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TOS:      8,
		TTL:      VRRPMultiTTL,
		TotalLen: ipv4.HeaderLen + len(payload),
		Protocol: VRRPIPProtocolNumber,
		Dst:      net.ParseIP(VRRPMultiAddr),
	}
	var cm = &ipv4.ControlMessage{IfIndex: iface.InterFace.Index}
	if errofwrite := iface.rawConn.WriteTo(ipheader, payload, cm); errofwrite != nil {
		return errofwrite
	}
	return nil
}

func (iface *IPv4Interface) ReadMessage() ([]byte, error) {
	for index := range iface.buffer {
		iface.buffer[index] = 0
	}
	if header, payload, _, err := iface.rawConn.ReadFrom(iface.buffer); err != nil {
		return nil, err
	} else {
		//pre check
		if header.Protocol != VRRPIPProtocolNumber {
			return nil, fmt.Errorf("VRRP IPv4 datagram protocol number %d", header.Protocol)
		}
		if header.TTL != VRRPMultiTTL {
			return nil, fmt.Errorf("VRRP IPv4 datagram TTL %d", header.TTL)
		}
		return payload, nil
	}
}

func NewIPv4Interface(ifname string) (interf *IPv4Interface, err error) {
	if c, err := net.ListenPacket(fmt.Sprintf("ip4:%d", VRRPIPProtocolNumber), "0.0.0.0"); err != nil {
		return nil, err
	} else {
		var RawC, errofraw = ipv4.NewRawConn(c)
		if errofraw != nil {
			return nil, errofraw
		}
		var interFace, errofifname = net.InterfaceByName(ifname)
		if errofifname != nil {
			return nil, errofifname
		}
		var VRRPAddr = net.IPAddr{IP: net.ParseIP(VRRPMultiAddr)}
		if errofjoin := RawC.JoinGroup(interFace, &VRRPAddr); errofjoin != nil {
			return nil, errofjoin
		}
		return &IPv4Interface{con: c, rawConn: RawC, InterFace: interFace, buffer: make([]byte, 65535)}, nil
	}
}
