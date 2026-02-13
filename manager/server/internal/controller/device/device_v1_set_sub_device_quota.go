package device

import (
	"context"

	"go-vnet/manager/server/api/device/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) SetSubDeviceQuota(ctx context.Context, req *v1.SetSubDeviceQuotaReq) (res *v1.SetSubDeviceQuotaRes, err error) {
	err = service.Device().SetSubDeviceQuota(ctx, req.SubDeviceId, req.Quota)
	if err != nil {
		return
	}
	res = &v1.SetSubDeviceQuotaRes{}
	return
}
