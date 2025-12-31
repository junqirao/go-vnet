package server

import (
	"go-vnet/server/network"

	"go-vnet/common/device"
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
