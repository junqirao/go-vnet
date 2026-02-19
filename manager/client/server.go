package client

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	"go-vnet/manager/client/internal/cmd"
	"go-vnet/manager/client/internal/service"
	"go-vnet/vnet/client"
)

func RunServer() {
	cmd.WebServer.Run(gctx.GetInitCtx())
}

func RegisterClientInstance(ins *client.Client) {
	service.Client().RegisterInstance(ins)
	g.Log().Infof(gctx.GetInitCtx(), "register client instance: %+v", ins)
}
