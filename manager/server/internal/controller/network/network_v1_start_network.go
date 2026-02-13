package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) StartNetwork(ctx context.Context, req *v1.StartNetworkReq) (res *v1.StartNetworkRes, err error) {
	err = service.Network().StartNetwork(ctx, req.NetworkId)
	res = new(v1.StartNetworkRes)
	return
}
