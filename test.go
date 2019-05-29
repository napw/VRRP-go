package main

import (
	"VRRP/VRRP"
	"fmt"
	"net"
	"time"
)

func main() {
	var vr = VRRP.NewVirtualRouter(240, "ens33", false, VRRP.IPv6)
	vr.SetPriorityAndMasterAdvInterval(243, time.Millisecond*700)
	vr.SetAdvInterval(time.Millisecond * 700)
	vr.SetPreemptMode(true)
	vr.AddIPvXAddr(net.ParseIP("fe80::e7ec:1b6e:8e59:c96b"))
	vr.AddIPvXAddr(net.ParseIP("fe80::e7ec:1b6e:8e59:c96a"))
	vr.Enroll(VRRP.Backup2Master, func() {
		fmt.Println("init to master")
	})
	vr.Enroll(VRRP.Master2Init, func() {
		fmt.Println("master to init")
	})
	vr.Enroll(VRRP.Master2Backup, func() {
		fmt.Println("master to backup")
	})
	go func() {
		time.Sleep(time.Second * 120)
		vr.Stop()
	}()
	vr.StartWithEventSelector()

}
