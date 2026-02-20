package client

import (
	"context"

	"go-vnet/manager/client/api/client/v1"
	"go-vnet/manager/client/internal/service"
)

func (c *ControllerV1) Stop(ctx context.Context, req *v1.StopReq) (res *v1.StopRes, err error) {
	err = service.Client().Stop(ctx)
	return
}
