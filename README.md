# VRRP-go
由golang实现的[VRRP-v3](https://tools.ietf.org/html/rfc5798), 点击超链接获取关于VRRP的信息。
[VRRP-v3](https://tools.ietf.org/html/rfc5798) implemented by golang，click hyperlink get details about VRRP

## example
```go
package main

import (
	"VRRP/VRRP"
	"flag"
	"fmt"
	"time"
)

var (
	VRID int
	Priority int
)

func init(){
	flag.IntVar(&VRID,"vrid",233,"virtual router ID")
	flag.IntVar(&Priority,"pri",100,"router priority")
}

func main() {
	flag.Parse()
	var vr = VRRP.NewVirtualRouter(byte(VRID), "ens33", false, VRRP.IPv4)
	vr.SetPriorityAndMasterAdvInterval(byte(Priority),time.Millisecond*800)
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
		time.Sleep(time.Minute * 5)
		vr.Stop()
	}()
	vr.StartWithEventSelector()

}
```
```shell
GOOS=linux go build -o vr test.go
#execute on host1
./vr -vrid=200 -pri=150
#execute on host2
./vr -vrid=200 -pri=230
```

## To-DO


