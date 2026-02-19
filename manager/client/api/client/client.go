// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package client

import (
	"context"

	"go-vnet/manager/client/api/client/v1"
)

type IClientV1 interface {
	GetRuntimeInfo(ctx context.Context, req *v1.GetRuntimeInfoReq) (res *v1.GetRuntimeInfoRes, err error)
}
