package server

import (
	"time"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	"go-vnet/manager/server/internal/cmd"
	_ "go-vnet/manager/server/internal/logic"
	_ "go-vnet/manager/server/internal/packed"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

type (
	managerServer struct {
		service.INetwork
		service.IDevice
		service.IQuota
	}
)

func RunServer() {
	cmd.WebServer.Run(gctx.GetInitCtx())
	go runSubmitFlowToDatabase()
}

func StartAllNetworks() {
	ctx := gctx.GetInitCtx()
	infos, err := service.Network().ListNetworkInfos(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "failed to get network infos: %s", err.Error())
		return
	}
	mgr := server.GetNetworkManager()
	for _, info := range infos {
		q, err := service.Quota().GetById(ctx, info.Quota)
		if err != nil {
			g.Log().Errorf(ctx, "failed to get network quota %s: %s", info.Id, err.Error())
			continue
		}
		n, err := server.NewNetwork(&server.NetworkConfig{
			ID:                   info.Id,
			CIDR:                 info.Cidr,
			MTU:                  info.Mtu,
			DataTrafficQuota:     int64(q.Value),
			DataTrafficQuotaType: q.Type,
		})
		if err != nil {
			g.Log().Errorf(ctx, "failed to start network %s: %s", info.Id, err.Error())
			continue
		}
		mgr.RegisterNetwork(n)
		g.Log().Infof(ctx, "network %s started", info.Id)
	}
}

func ManagerServer() server.ManagerServer {
	return &managerServer{
		INetwork: service.Network(),
		IDevice:  service.Device(),
		IQuota:   service.Quota(),
	}
}

func runSubmitFlowToDatabase() {
	ctx := gctx.GetInitCtx()
	for {
		time.Sleep(time.Minute * 1)
		err := service.Quota().SubmitFlowToDatabase(ctx)
		if err != nil {
			g.Log().Errorf(ctx, "failed to submit flow to database: %s", err.Error())
		}
	}
}
