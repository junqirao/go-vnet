package generic

import (
	"context"

	"go-vnet/manager/server/api/generic/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) GetGenericInfo(ctx context.Context, req *v1.GetGenericInfoReq) (res *v1.GetGenericInfoRes, err error) {
	data, err := service.Generic().GetGenericInfo(ctx, req.Period)
	if err != nil {
		return
	}
	res = (*v1.GetGenericInfoRes)(data)
	return
}
