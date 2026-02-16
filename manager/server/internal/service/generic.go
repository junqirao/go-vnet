// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"go-vnet/manager/server/internal/model"
	"go-vnet/vnet/server"
)

type (
	IGeneric interface {
		GetGenericInfo(ctx context.Context) (info *model.GenericInfo, err error)
		Bind(srv *server.Server)
	}
)

var (
	localGeneric IGeneric
)

func Generic() IGeneric {
	if localGeneric == nil {
		panic("implement not found for interface IGeneric, forgot register?")
	}
	return localGeneric
}

func RegisterGeneric(i IGeneric) {
	localGeneric = i
}
