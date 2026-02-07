// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package quota

import (
	"context"

	"go-vnet/manager/server/api/quota/v1"
)

type IQuotaV1 interface {
	GetQuota(ctx context.Context, req *v1.GetQuotaReq) (res *v1.GetQuotaRes, err error)
	ListQuota(ctx context.Context, req *v1.ListQuotaReq) (res *v1.ListQuotaRes, err error)
	CreateQuota(ctx context.Context, req *v1.CreateQuotaReq) (res *v1.CreateQuotaRes, err error)
	UpdateQuota(ctx context.Context, req *v1.UpdateQuotaReq) (res *v1.UpdateQuotaRes, err error)
	DeleteQuota(ctx context.Context, req *v1.DeleteQuotaReq) (res *v1.DeleteQuotaRes, err error)
}
