package server

import (
	"context"

	"go-vnet/common/addresses"
	"go-vnet/common/auth"
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
		auth            auth.AuthorizedHandler
		allocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *model.Device, err error)
		deviceSignFunc  func(d *model.Device)
	}
	NetworkConfig struct {
		ID         string `json:"id"`
		CIDR       string `json:"cidr"`
		RouterData []byte `json:"router_data"`
	}
)

func NewNetwork(cfg *NetworkConfig, auth auth.AuthorizedHandler) (n *Network, err error) {
	allocator, err := addresses.NewIPAllocator(cfg.CIDR)
	if err != nil {
		return
	}
	n = &Network{
		ID:     cfg.ID,
		CIDR:   cfg.CIDR,
		router: router.NewRouter(cfg.RouterData),
		pool:   allocator,
		auth:   auth,
	}
	return
}

func (n *Network) Router() router.Router {
	return n.router
}

func (n *Network) AcquireDevice(ctx context.Context, in []byte) (dev *model.Device, err error) {
	payload, err := n.auth.Handle(ctx, in)
	if err != nil {
		return
	}

	dev = &model.Device{}
	if n.allocDeviceFunc != nil {
		if dev, err = n.allocDeviceFunc(ctx, payload); err != nil {
			return
		}
	}

	if dev.CIDR != "" {
		err = n.pool.AssignSpecific(dev.CIDR)
	} else {
		dev.CIDR, err = n.pool.AssignRandom()
	}
	if err != nil {
		return
	}

	if n.deviceSignFunc != nil {
		n.deviceSignFunc(dev)
	}
	return
}
