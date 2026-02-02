// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"go-vnet/manager/server/internal/model"
)

type (
	INetwork interface {
		GetNetworkDetails(_ context.Context, networkId string) (details *model.NetworkDetails, err error)
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
