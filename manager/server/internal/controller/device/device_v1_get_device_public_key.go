package device

import (
	"context"

	"go-vnet/manager/server/api/device/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) GetDevicePublicKey(ctx context.Context, req *v1.GetDevicePublicKeyReq) (res *v1.GetDevicePublicKeyRes, err error) {
	pk, err := service.Device().DownloadDevicePubKey(ctx, req.DeviceId)
	if err != nil {
		return
	}
	res = &v1.GetDevicePublicKeyRes{
		PublicKey: pk,
	}
	return
}
