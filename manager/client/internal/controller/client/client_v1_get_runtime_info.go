package client

import (
	"context"

	"go-vnet/manager/client/api/client/v1"
	"go-vnet/manager/client/internal/service"
)

func (c *ControllerV1) GetRuntimeInfo(ctx context.Context, _ *v1.GetRuntimeInfoReq) (res *v1.GetRuntimeInfoRes, err error) {
	data, err := service.Client().GetRuntimeInfo(ctx)
	if err != nil {
		return
	}
	res = (*v1.GetRuntimeInfoRes)(data)
	return
}
