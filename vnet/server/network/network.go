package network

import (
	"context"

	"go-vnet/common/addresses"
	"go-vnet/common/device"
	"go-vnet/common/flow"
	"go-vnet/vnet/router"
)

type (
	Network struct {
		Config          `json:"config"`
		router          *router.Router
		pool            *addresses.IPAllocator
		control         *flow.Control
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *device.Device, err error)
	}
	Config struct {
		ID              string `json:"id"`
		CIDR            string `json:"cidr"`
		MTU             int    `json:"mtu"`
		RouterData      []byte `json:"-"`
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *device.Device, err error)
		deviceSignFunc  func(d *device.IDevice)
	}
)

func NewNetwork(cfg *Config) (n *Network, err error) {
	allocator, err := addresses.NewIPAllocator(cfg.CIDR)
	if err != nil {
		return
	}
	n = &Network{
		Config: *cfg,
		router: router.NewRouter(),
		pool:   allocator,
	}
	return
}

func (n *Network) Router() *router.Router {
	return n.router
}

func (n *Network) AcquireDevice(ctx context.Context, request map[string]any) (dev *device.Device, err error) {
	dev = &device.Device{}
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
	return
}

func (n *Network) ReleaseDevice(cidr string) (err error) {
	return n.pool.ReleaseIP(cidr)
}

func (n *Network) Control() *flow.Control {
	return n.control
}

func (n *Network) SetControl(control *flow.Control) {
	n.control = control
}
