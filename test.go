package main

import (
	"VRRP/VRRP"
	"fmt"
	"net"
	"time"
)

func main() {
	var vr = VRRP.NewVirtualRouter(240, "ens33", false, VRRP.IPv4)
	vr.AddIPvXAddr(net.ParseIP("192.168.83.24"))
	vr.AddIPvXAddr(net.ParseIP("192.168.83.24"))
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
