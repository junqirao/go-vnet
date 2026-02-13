package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) UpdateNetwork(ctx context.Context, req *v1.UpdateNetworkReq) (res *v1.UpdateNetworkRes, err error) {
	err = service.Network().UpdateNetwork(ctx, req.NetworkId, req.Fields)
	res = new(v1.UpdateNetworkRes)
	return
}
