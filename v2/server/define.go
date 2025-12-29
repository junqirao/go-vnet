package server

import (
	"go-vnet/common/device"
	"go-vnet/server/network"
)

type (
	Session struct {
		SendReceiver
		Id               string
		Network          *network.Network
		DispatchedDevice *device.Device
		IP               string
	}
)
