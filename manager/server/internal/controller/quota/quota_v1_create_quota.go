package quota

import (
	"context"

	"go-vnet/manager/server/api/quota/v1"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) CreateQuota(ctx context.Context, req *v1.CreateQuotaReq) (res *v1.CreateQuotaRes, err error) {
	err = service.Quota().Create(ctx, &entity.Quota{
		Name:       req.Name,
		Type:       req.Type,
		Value:      req.Value,
		Period:     req.Period,
		Target:     req.Target,
		TargetType: req.TargetType,
	})
	if err != nil {
		return
	}
	res = &v1.CreateQuotaRes{}
	return
}
