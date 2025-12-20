package main

import (
	"fmt"
	"time"

	"device"
)

func main() {
	dev := device.NewTunDevice(device.Config{
		Name: "test-device",
		CIDR: "192.168.4.2/32",
		MTU:  1500,
	})
	err := dev.Setup()
	if err != nil {
		println(err.Error())
		return
	}
	fmt.Println("create device success, exit in 30s")
	time.Sleep(time.Second * 30)
}
