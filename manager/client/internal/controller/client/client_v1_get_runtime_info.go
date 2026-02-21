package client

import (
	"context"
	"time"

	"go-vnet/manager/client/api/client/v1"
	"go-vnet/manager/client/internal/service"
)

func (c *ControllerV1) GetRuntimeInfo(ctx context.Context, req *v1.GetRuntimeInfoReq) (res *v1.GetRuntimeInfoRes, err error) {
	duration, err := time.ParseDuration(req.Duration)
	if err != nil {
		duration = 0
	}
	data, err := service.Client().GetRuntimeInfo(ctx, duration)
	if err != nil {
		return
	}
	res = (*v1.GetRuntimeInfoRes)(data)
	return
}
