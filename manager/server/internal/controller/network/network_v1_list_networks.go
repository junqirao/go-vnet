package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) ListNetworks(ctx context.Context, req *v1.ListNetworksReq) (res *v1.ListNetworksRes, err error) {
	list, total, err := service.Network().ListNetworks(ctx, req.Page, req.PageSize, req.Name)
	if err != nil {
		return
	}
	res = &v1.ListNetworksRes{
		List:  list,
		Total: total,
		Page:  req.Page,
	}
	return
}
