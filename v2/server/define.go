package server

import (
	"go-vnet/common/device"
	"go-vnet/server/network"
)

type (
	Session struct {
		SendReceiver
		Type             SessionType
		Id               string
		Network          *network.Network
		DispatchedDevice *device.Device
		IP               string
	}
	SessionType = Type
)
