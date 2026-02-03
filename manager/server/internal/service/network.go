// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
)

type (
	INetwork interface {
		GetNetworkDetails(ctx context.Context, networkId string) (details *model.NetworkDetails, err error)
		CreateNetwork(ctx context.Context, network *entity.Network) (id string, err error)
		StopNetwork(ctx context.Context, networkId string) (err error)
		StartNetwork(ctx context.Context, networkId string) (err error)
		DeleteNetwork(ctx context.Context, networkId string) (err error)
		UpdateNetwork(ctx context.Context, networkId string, fields map[string]any) (err error)
		ListNetworkInfos(ctx context.Context) (ns []*entity.Network, err error)
		ListSessionByNetwork(_ context.Context, networkId string) (sessions []*model.Session, err error)
	}
)

var (
	localNetwork INetwork
)

func Network() INetwork {
	if localNetwork == nil {
		panic("implement not found for interface INetwork, forgot register?")
	}
	return localNetwork
}

func RegisterNetwork(i INetwork) {
	localNetwork = i
}
