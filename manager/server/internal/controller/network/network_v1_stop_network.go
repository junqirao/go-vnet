package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) StopNetwork(ctx context.Context, req *v1.StopNetworkReq) (res *v1.StopNetworkRes, err error) {
	err = service.Network().StopNetwork(ctx, req.NetworkId)
	res = new(v1.StopNetworkRes)
	return
}
