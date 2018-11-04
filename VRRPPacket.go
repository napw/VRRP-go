package main

import "fmt"

type VRRPPacket struct {
	Header [8]byte
	IPAddress [][4]byte
}

func (packet *VRRPPacket)GetVersion()byte{
	return packet.Header[0]&15
}

func (packet *VRRPPacket)GetType()byte{
	return packet.Header[0]&240
}

func (packet *VRRPPacket)GetVirtualRouterID()byte{
	return packet.Header[1]
}

func (packet *VRRPPacket)GetPriority()byte{
	return packet.Header[2]
}

func (packet *VRRPPacket)GetIPvXAddrCount()byte{
	return packet.Header[3]
}

func (packet *VRRPPacket)GetAdvertisementInterval()int32{
	return int32(packet.Header[4]&240+packet.Header[5]*16)
}

func (packet *VRRPPacket)GetCheckSum()int32{
	return int32(packet.Header[2])+256*int32(packet.Header[3])
}

func main(){
	fmt.Print()
}
