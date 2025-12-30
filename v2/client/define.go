package client

import (
	"go-vnet/common/device"
)

type (
	JoinNetworkResponse struct {
		Device device.Config `json:"device"`
	}
	Session struct {
		SendReceiver
		DispatchedDevice device.Config `json:"dispatched_device"`
	}
)
