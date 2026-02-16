// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package generic

import (
	"context"

	"go-vnet/manager/server/api/generic/v1"
)

type IGenericV1 interface {
	GetGenericInfo(ctx context.Context, req *v1.GetGenericInfoReq) (res *v1.GetGenericInfoRes, err error)
}
