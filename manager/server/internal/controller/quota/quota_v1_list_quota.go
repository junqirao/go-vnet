package quota

import (
	"context"

	"go-vnet/manager/server/api/quota/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) ListQuota(ctx context.Context, req *v1.ListQuotaReq) (res *v1.ListQuotaRes, err error) {
	data, err := service.Quota().List(ctx, req.Type)
	if err != nil {
		return
	}
	res = &v1.ListQuotaRes{Quotas: data}
	return
}
