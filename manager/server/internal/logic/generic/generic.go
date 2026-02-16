package generic

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func init() {
	service.RegisterGeneric(new(sGeneric))
}

type sGeneric struct {
	s *server.Server
}

func (s *sGeneric) GetGenericInfo(_ context.Context) (info *model.GenericInfo, err error) {
	networks := server.GetNetworkManager().GetAllNetworks()

	info = &model.GenericInfo{
		Server: model.ServerGenericInfo{},
		P2P:    model.P2PGenericInfo{},
		Network: model.NetworkGenericInfo{
			Running: len(networks),
		},
	}
	for _, network := range networks {
		ss := network.ListSessions()
		for _, session := range ss {
			session.Proxying.Range(func(key, value any) bool {
				info.Network.Connections++
				return true
			})
			info.Network.TotalProxyRxBytes += session.Metrics.RxBytes.Total.Load()
			info.Network.TotalProxyRxSpeed += session.Metrics.RxBytes.Speed.Load()
			info.Network.TotalProxyTxBytes += session.Metrics.TxBytes.Total.Load()
			info.Network.TotalProxyTxSpeed += session.Metrics.TxBytes.Speed.Load()
			info.Network.TotalProxyRxPackets += session.Metrics.RxPackets.Total.Load()
			info.Network.TotalProxyTxPackets += session.Metrics.TxPackets.Total.Load()
		}
		info.Network.Sessions += len(ss)
	}

	info.P2P.Running = s.s.P2PSignalingServerHost().Peerstore().Peers().Len() - 1
	info.Server.Transports = s.s.Config().Servers
	return
}

func (s *sGeneric) Bind(srv *server.Server) {
	s.s = srv
	g.Log().Info(gctx.GetInitCtx(), "generic service bind server success")
}
