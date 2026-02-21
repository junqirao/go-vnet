package client

import (
	"context"
	"time"

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

func (s *sClient) GetRuntimeInfo(_ context.Context, duration time.Duration) (res *client.RuntimeInfo, err error) {
	if s.ins == nil {
		err = response.CodeNotFound.WithDetail("client instance not registered")
		return
	}
	res = s.ins.CollectRuntimeInfo(duration)
	if res.Session != nil {
		res.Session.DispatchedDevice.Key = ""
	}
	return
}

func (s *sClient) Stop(ctx context.Context) (err error) {
	if s.ins == nil {
		err = response.CodeNotFound.WithDetail("client instance not registered")
		return
	}
	return s.ins.Stop(ctx)
}

func (s *sClient) Resume(ctx context.Context) (err error) {
	if s.ins == nil {
		err = response.CodeNotFound.WithDetail("client instance not registered")
		return
	}
	return s.ins.Resume(ctx)
}
