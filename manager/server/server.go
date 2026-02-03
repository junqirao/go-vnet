package server

import (
	"github.com/gogf/gf/v2/os/gctx"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"

	"go-vnet/common/logger"
	"go-vnet/manager/server/internal/cmd"
	_ "go-vnet/manager/server/internal/logic"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func RunServer() {
	StartAllNetworks()
	cmd.WebServer.Run(gctx.GetInitCtx())
}

func StartAllNetworks() {
	ctx := gctx.GetInitCtx()
	infos, err := service.Network().ListNetworkInfos(ctx)
	if err != nil {
		logger.DefaultLogger.Errorf(ctx, "failed to get network infos: %s", err.Error())
		return
	}
	mgr := server.GetNetworkManager()
	for _, info := range infos {
		n, err := server.NewNetwork(&server.NetworkConfig{
			ID:   info.Id,
			CIDR: info.Cidr,
			MTU:  info.Mtu,
		})
		if err != nil {
			logger.DefaultLogger.Errorf(ctx, "failed to start network %s: %s", info.Id, err.Error())
			continue
		}
		mgr.RegisterNetwork(n)
		logger.DefaultLogger.Errorf(ctx, "network %s started", info.Id)
	}
}
