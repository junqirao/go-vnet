package client

import (
	"context"

	"go-vnet/manager/client/api/client/v1"
	"go-vnet/manager/client/internal/service"
)

func (c *ControllerV1) Resume(ctx context.Context, req *v1.ResumeReq) (res *v1.ResumeRes, err error) {
	err = service.Client().Resume(ctx)
	return
}
