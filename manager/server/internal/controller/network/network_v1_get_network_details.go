package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) GetNetworkDetails(ctx context.Context, req *v1.GetNetworkDetailsReq) (res *v1.GetNetworkDetailsRes, err error) {
	data, err := service.Network().GetNetworkDetails(ctx, req.NetworkId)
	if err != nil {
		return
	}
	res = (*v1.GetNetworkDetailsRes)(data)
	return
}
