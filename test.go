package main

import (
	"VRRP/VRRP"
	"net"
)

func main() {
	/*var t, err = network.NewIPv4Interface("ens33")
	if err != nil {
		fmt.Printf("err occurred when create if, %v", err)
	} else {
		for {
			var p VRRP.VRRPPacket
			p.SetAdvertisementInterval(1)
			p.SetPriority(250)
			p.SetVirtualRouterID(67)
			p.SetVersion(3)
			p.SetType()
			p.SetIPvXAddr(4, net.IPv4(192, 34, 54, 56))
			p.SetIPvXAddr(4, net.IPv4(192, 34, 54, 58))
			p.SetIPvXAddr(4, net.IPv4(156, 34, 54, 58))
			var pp = &VRRP.PseudoHeader{Saddr: net.IPv4(192, 168, 83, 135), Daddr: net.IPv4(224, 0, 0, 18), Protocol: 112, Len: uint16(len(p.ToBytes()))}
			p.SetCheckSum(pp)
			if err := t.WriteMessage(p.ToBytes()); err != nil {
				fmt.Printf("error occurred when send, %v\n", err)
			}
			time.Sleep(5 * time.Second)
		}

	}*/
	var vr = VRRP.NewVirtualRouter(200, "ens33", false, VRRP.IPv4)
	vr.SetPriority(150)
	vr.SetMasterAdvInterval(50)
	vr.SetAdvInterval(50)
	vr.SetPreemptMode(false)
	vr.AddIPvXAddr(net.IPv4(1, 1, 1, 1))
	vr.AddIPvXAddr(net.IPv4(172, 23, 27, 199))
	vr.State = VRRP.MASTER
	for i := 0; i < 3; i++ {
		vr.EventLoop()
	}

}
