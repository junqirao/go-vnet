package device

import (
	"context"

	"go-vnet/manager/server/api/device/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) CreateSubDevice(ctx context.Context, req *v1.CreateSubDeviceReq) (res *v1.CreateSubDeviceRes, err error) {
	res = new(v1.CreateSubDeviceRes)
	err = service.Device().CreateSubDevice(ctx, req.DeviceId, req.NetworkId, req.Quota, req.Settings)
	return
}
