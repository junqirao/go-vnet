package quota

import (
	"context"

	"go-vnet/manager/server/api/quota/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) GetQuota(ctx context.Context, req *v1.GetQuotaReq) (res *v1.GetQuotaRes, err error) {
	data, err := service.Quota().GetById(ctx, req.Id)
	if err != nil {
		return
	}
	res = (*v1.GetQuotaRes)(data)
	return
}
