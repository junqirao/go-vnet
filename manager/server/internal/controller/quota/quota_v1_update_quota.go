package quota

import (
	"context"

	"go-vnet/manager/server/api/quota/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) UpdateQuota(ctx context.Context, req *v1.UpdateQuotaReq) (res *v1.UpdateQuotaRes, err error) {
	err = service.Quota().Update(ctx, req.Id, req.Fields)
	if err != nil {
		return
	}
	res = &v1.UpdateQuotaRes{}
	return
}
