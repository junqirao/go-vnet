package generic

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/gtime"

	"go-vnet/common/quota"
	"go-vnet/manager/server/internal/dao"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func init() {
	service.RegisterGeneric(new(sGeneric))
}

type sGeneric struct {
	s     *server.Server
	cache *gcache.Cache
}

func (s *sGeneric) GetGenericInfo(ctx context.Context, period string) (info *model.GenericInfo, err error) {
	networks := server.GetNetworkManager().GetAllNetworks()

	info = &model.GenericInfo{
		Server: model.ServerGenericInfo{},
		P2P:    model.P2PGenericInfo{},
		Network: model.NetworkGenericInfo{
			Running: len(networks),
		},
	}

	m := dao.QuotaFlow.Ctx(ctx)
	date := gtime.Now()
	switch period {
	case "total":
	case "week":
		// 本周第一天~最后一天
		m = m.WhereGTE(dao.QuotaFlow.Columns().RecordStart, date.StartOfWeek().Unix())
		m = m.WhereLTE(dao.QuotaFlow.Columns().RecordEnd, date.EndOfWeek().Unix())
	case "month":
		// 本月第一天~最后一天
		m = m.WhereGTE(dao.QuotaFlow.Columns().RecordStart, date.StartOfMonth().Unix())
		m = m.WhereLTE(dao.QuotaFlow.Columns().RecordEnd, date.EndOfMonth().Unix())
	default:
		// day
		// 00:00:00~23:59:59
		m = m.WhereGTE(dao.QuotaFlow.Columns().RecordStart, date.StartOfDay().Unix())
		m = m.WhereLTE(dao.QuotaFlow.Columns().RecordEnd, date.EndOfDay().Unix())
	}
	flows, err := m.Cache(gdb.CacheOption{
		Duration: time.Minute * 5,
		Name:     "flow_by_period",
	}).Where(dao.QuotaFlow.Columns().TargetType, quota.TargetTypeDeviceTraffic).All()
	if err != nil {
		return
	}
	for _, flow := range flows {
		e := &entity.QuotaFlow{}
		_ = flow.Struct(e)
		info.Network.TotalProxyBytes += uint64(e.Usage)
	}
	for _, network := range networks {
		ss := network.ListSessions()
		for _, session := range ss {
			session.Proxying.Range(func(key, value any) bool {
				info.Network.Connections++
				return true
			})
			info.Network.TotalProxyBytes += session.Metrics.RxBytes.Total.Load()
			info.Network.TotalProxyBytes += session.Metrics.TxBytes.Total.Load()
			info.Network.TotalProxyRxSpeed += session.Metrics.RxBytes.Speed.Load()
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
