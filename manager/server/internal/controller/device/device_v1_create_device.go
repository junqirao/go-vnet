package device

import (
	"context"

	"go-vnet/manager/server/api/device/v1"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) CreateDevice(ctx context.Context, req *v1.CreateDeviceReq) (res *v1.CreateDeviceRes, err error) {
	res = new(v1.CreateDeviceRes)
	res.Id, err = service.Device().CreateDevice(ctx, req.Key, &entity.Device{
		Name: req.Name,
	})
	return
}
