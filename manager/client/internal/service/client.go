// ================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// You can delete these comments if you wish manually maintain this interface file.
// ================================================================================

package service

import (
	"context"
	"go-vnet/vnet/client"
)

type (
	IClient interface {
		RegisterInstance(c *client.Client)
		GetRuntimeInfo(_ context.Context) (res *client.RuntimeInfo, err error)
	}
)

var (
	localClient IClient
)

func Client() IClient {
	if localClient == nil {
		panic("implement not found for interface IClient, forgot register?")
	}
	return localClient
}

func RegisterClient(i IClient) {
	localClient = i
}
