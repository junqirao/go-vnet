package network

import (
	"context"

	"go-vnet/common/addresses"
	device2 "go-vnet/common/device"
	"go-vnet/common/router"
)

type (
	Network struct {
		Config
		// todo 分布式支持
		router          router.Router
		pool            *addresses.IPAllocator
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *device2.Device, err error)
	}
	Config struct {
		ID              string `json:"id"`
		CIDR            string `json:"cidr"`
		MTU             int    `json:"mtu"`
		RouterData      []byte `json:"router_data"`
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *device2.Device, err error)
		deviceSignFunc  func(d *device2.IDevice)
	}
)

func NewNetwork(cfg *Config) (n *Network, err error) {
	allocator, err := addresses.NewIPAllocator(cfg.CIDR)
	if err != nil {
		return
	}
	n = &Network{
		Config: *cfg,
		router: router.NewRouter(cfg.RouterData),
		pool:   allocator,
	}
	return
}

func (n *Network) Router() router.Router {
	return n.router
}

func (n *Network) AcquireDevice(ctx context.Context, request map[string]any) (dev *device2.Device, err error) {
	dev = &device2.Device{}
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

func (n *Network) ReleaseDevice(dev *device2.Device) (err error) {
	return n.pool.ReleaseIP(dev.CIDR)
}
