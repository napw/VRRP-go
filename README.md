# VRRP-go
由golang实现的[VRRP-v3](https://tools.ietf.org/html/rfc5798), 点击超链接获取关于VRRP的信息。
[VRRP-v3](https://tools.ietf.org/html/rfc5798) implemented by golang，click hyperlink get details about VRRP

## example
```go
    package main
    
    import (
    	"VRRP/VRRP"
    	"net"
    )
    
    func main() {
    	var vr = VRRP.NewVirtualRouter(200, "ens33", false, VRRP.IPv6)
    	vr.SetPriority(100)
    	vr.SetMasterAdvInterval(50)
    	vr.SetAdvInterval(50)
    	vr.SetPreemptMode(true)
    	vr.AddIPvXAddr(net.ParseIP("fe80::e7ec:1b6e:8e59:c96b"))
    	vr.AddIPvXAddr(net.ParseIP("fe80::e7ec:1b6e:8e59:c96a"))
    	go vr.FetchVRRPPacket()
    	go func() {
    		vr.EventChannel<-VRRP.START
    	}()
    	for {
    		vr.EventLoop()
    	} 
    }
```

## To-DO
1. add callback for state switching 为状态切换添加回调
2. reduce CPU usage 降低CPU使用率
3. more comprehensive example 更详细的示例代码
