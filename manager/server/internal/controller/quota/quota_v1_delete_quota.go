package quota

import (
	"context"

	"go-vnet/manager/server/api/quota/v1"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) DeleteQuota(ctx context.Context, req *v1.DeleteQuotaReq) (res *v1.DeleteQuotaRes, err error) {
	err = service.Quota().Delete(ctx, req.Id)
	if err != nil {
		return
	}
	res = &v1.DeleteQuotaRes{}
	return
}
