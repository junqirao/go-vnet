package client

import (
	"context"

	"github.com/junqirao/gocomponents/response"

	"go-vnet/manager/client/internal/service"
	"go-vnet/vnet/client"
)

func init() {
	service.RegisterClient(&sClient{})
}

type sClient struct {
	ins *client.Client
}

func (s *sClient) RegisterInstance(c *client.Client) {
	s.ins = c
}

func (s *sClient) GetRuntimeInfo(_ context.Context) (res *client.RuntimeInfo, err error) {
	if s.ins == nil {
		err = response.CodeNotFound.WithDetail("client instance not registered")
		return
	}
	res = s.ins.CollectRuntimeInfo()
	res.Session.DispatchedDevice.Key = ""
	return
}
