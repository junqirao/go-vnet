package generic

import (
	"context"

	"go-vnet/manager/server/api/generic/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) GetGenericInfo(ctx context.Context, _ *v1.GetGenericInfoReq) (res *v1.GetGenericInfoRes, err error) {
	data, err := service.Generic().GetGenericInfo(ctx)
	if err != nil {
		return
	}
	res = (*v1.GetGenericInfoRes)(data)
	return
}
