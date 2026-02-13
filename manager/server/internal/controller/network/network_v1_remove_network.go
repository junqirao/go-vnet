package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) RemoveNetwork(ctx context.Context, req *v1.RemoveNetworkReq) (res *v1.RemoveNetworkRes, err error) {
	err = service.Network().DeleteNetwork(ctx, req.NetworkId)
	res = new(v1.RemoveNetworkRes)
	return
}
