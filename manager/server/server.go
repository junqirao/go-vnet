package server

import (
	"github.com/gogf/gf/v2/os/gctx"

	"go-vnet/manager/server/internal/cmd"
	_ "go-vnet/manager/server/internal/logic"
)

func RunServer() {
	cmd.WebServer.Run(gctx.GetInitCtx())
}
