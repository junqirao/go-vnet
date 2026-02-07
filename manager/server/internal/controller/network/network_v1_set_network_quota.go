package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) SetNetworkQuota(ctx context.Context, req *v1.SetNetworkQuotaReq) (res *v1.SetNetworkQuotaRes, err error) {
	err = service.Network().SetNetworkQuota(ctx, req.NetworkId, req.Quota)
	if err != nil {
		return
	}
	res = &v1.SetNetworkQuotaRes{}
	return
}
