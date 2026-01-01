package client

import (
	"go-vnet/common/device"
	"go-vnet/common/session"
	"go-vnet/server/network"
)

type (
	JoinNetworkResponse struct {
		Device  device.Config `json:"device"`
		Session serverSession `json:"session"`
	}
	serverSession struct {
		session.Session `json:"session"`
		Network         struct {
			network.Config `json:"config"`
		} `json:"network"`
	}
)
