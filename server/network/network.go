package network

import (
	"context"

	"go-vnet/common/addresses"
	"go-vnet/common/router"
	"go-vnet/model"
)

type (
	Network struct {
		ID   string `json:"id"`
		CIDR string `json:"cidr"`

		// todo 分布式支持
		router          router.Router
		pool            *addresses.IPAllocator
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *model.Device, err error)
	}
	Config struct {
		ID              string `json:"id"`
		CIDR            string `json:"cidr"`
		RouterData      []byte `json:"router_data"`
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *model.Device, err error)
		deviceSignFunc  func(d *model.Device)
	}
)

func NewNetwork(cfg *Config) (n *Network, err error) {
	allocator, err := addresses.NewIPAllocator(cfg.CIDR)
	if err != nil {
		return
	}
	n = &Network{
		ID:     cfg.ID,
		CIDR:   cfg.CIDR,
		router: router.NewRouter(cfg.RouterData),
		pool:   allocator,
	}
	return
}

func (n *Network) Router() router.Router {
	return n.router
}

func (n *Network) AcquireDevice(ctx context.Context, request map[string]any) (dev *model.Device, err error) {
	dev = &model.Device{}
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
	return
}
