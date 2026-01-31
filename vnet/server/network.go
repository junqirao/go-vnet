package server

import (
	"context"
	"sync"

	"golang.zx2c4.com/wireguard/device"

	"go-vnet/common/addresses"
	"go-vnet/common/flow"
	"go-vnet/common/router"
	"go-vnet/common/session"
)

type (
	Network struct {
		NetworkConfig   `json:"config"`
		sessions        sync.Map // cidr : *Session
		router          *router.Router
		pool            *addresses.IPAllocator
		control         *flow.Control
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *session.Device, err error)
	}
	NetworkConfig struct {
		ID              string `json:"id"`
		CIDR            string `json:"cidr"`
		MTU             int    `json:"mtu"`
		RouterData      []byte `json:"-"`
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *device.Device, err error)
	}
)

func NewNetwork(cfg *NetworkConfig) (n *Network, err error) {
	allocator, err := addresses.NewIPAllocator(cfg.CIDR)
	if err != nil {
		return
	}
	n = &Network{
		NetworkConfig: *cfg,
		router:        router.NewRouter(),
		pool:          allocator,
	}
	return
}

func (n *Network) Router() *router.Router {
	return n.router
}

func (n *Network) AcquireDevice(ctx context.Context, sess *Session, request map[string]any) (dev *session.Device, err error) {
	dev = &session.Device{}
	if n.allocDeviceFunc != nil {
		if dev, err = n.allocDeviceFunc(ctx, request); err != nil {
			return
		}
	}

	if dev.CIDR != "" {
		err = n.pool.AssignSpecific(dev.CIDR)
	} else {
		dev.CIDR, err = n.pool.AssignRandom()
	}
	if dev.MTU == 0 {
		dev.MTU = n.MTU
	}

	n.sessions.Store(dev.CIDR, sess)
	return
}

func (n *Network) ReleaseDevice(cidr string) (err error) {
	n.sessions.Delete(cidr)
	return n.pool.ReleaseIP(cidr)
}

func (n *Network) Control() *flow.Control {
	return n.control
}

func (n *Network) SetControl(control *flow.Control) {
	n.control = control
}
