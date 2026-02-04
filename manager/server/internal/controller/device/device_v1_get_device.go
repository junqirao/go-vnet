package device

import (
	"context"

	"go-vnet/manager/server/api/device/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) GetDevice(ctx context.Context, req *v1.GetDeviceReq) (res *v1.GetDeviceRes, err error) {
	info, err := service.Device().GetDeviceInfo(ctx, req.DeviceId)
	if err != nil {
		return
	}
	res = (*v1.GetDeviceRes)(info)
	return
}
