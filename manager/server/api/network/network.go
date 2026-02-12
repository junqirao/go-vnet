// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package network

import (
	"context"

	"go-vnet/manager/server/api/network/v1"
)

type INetworkV1 interface {
	GetNetworkDetails(ctx context.Context, req *v1.GetNetworkDetailsReq) (res *v1.GetNetworkDetailsRes, err error)
	CreateNetwork(ctx context.Context, req *v1.CreateNetworkReq) (res *v1.CreateNetworkRes, err error)
	StopNetwork(ctx context.Context, req *v1.StopNetworkReq) (res *v1.StopNetworkRes, err error)
	StartNetwork(ctx context.Context, req *v1.StartNetworkReq) (res *v1.StartNetworkRes, err error)
	RemoveNetwork(ctx context.Context, req *v1.RemoveNetworkReq) (res *v1.RemoveNetworkRes, err error)
	UpdateNetwork(ctx context.Context, req *v1.UpdateNetworkReq) (res *v1.UpdateNetworkRes, err error)
	ListSession(ctx context.Context, req *v1.ListSessionReq) (res *v1.ListSessionRes, err error)
}
