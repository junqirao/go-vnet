package device

import (
	"context"

	"go-vnet/manager/server/api/device/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) SetSubDeviceBandwidthQuota(ctx context.Context, req *v1.SetSubDeviceBandwidthQuotaReq) (res *v1.SetSubDeviceBandwidthQuotaRes, err error) {
	err = service.Device().SetSubDeviceBandwidthQuota(ctx, req.SubDeviceId, req.BandwidthQuota)
	if err != nil {
		return
	}
	res = &v1.SetSubDeviceBandwidthQuotaRes{}
	return
}
