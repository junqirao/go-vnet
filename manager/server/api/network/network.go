// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
)

type INetworkV1 interface {
	ListSession(ctx context.Context, req *v1.ListSessionReq) (res *v1.ListSessionRes, err error)
}
