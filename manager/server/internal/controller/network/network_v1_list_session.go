package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) ListSession(ctx context.Context, req *v1.ListSessionReq) (res *v1.ListSessionRes, err error) {
	sessions, err := service.Network().ListSessionByNetwork(ctx, req.NetworkId)
	if err != nil {
		return
	}
	res = &v1.ListSessionRes{
		Sessions: sessions,
	}
	return
}
