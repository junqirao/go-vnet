package client

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/vnet/client/hub"
)

func (c *Client) State() State {
	return c.state
}

func (c *Client) CollectRuntimeInfo(recordDuration time.Duration) (info *RuntimeInfo) {
	now := time.Now()
	if recordDuration == 0 {
		recordDuration = time.Minute * 5
	}
	start := now.Add(-recordDuration).Unix()
	end := now.Unix()
	info = &RuntimeInfo{
		State:          c.state.String(),
		Mode:           c.mode.String(),
		Config:         c.cfg,
		Metrics:        c.transport.metrics,
		MetricsRecords: c.transport.localMetricsDB.GetRange(start, end),
	}
	info.Heartbeat = &HeartbeatRuntimeInfo{
		LatencyToServer:   c.ping.latencyToServer,
		ErrorCount:        c.ping.errorCount,
		ReconnectInterval: c.ping.reconnectInterval,
		LastHeartbeat:     c.ping.lastHeartbeat,
		Retries:           c.ping.retires,
	}
	if c.state != StateRunning {
		return
	}
	info.Session = c.session
	info.P2P = &P2PRuntimeInfo{
		HostId:         c.p2p.hostId,
		Metrics:        c.p2p.metrics,
		Connections:    []string{},
		MetricsRecords: c.p2p.localMetricsDB.GetRange(start, end),
	}
	if c.p2p.router != nil {
		keys := c.p2p.router.Keys()
		// remove self
		for i, key := range keys {
			if strings.HasPrefix(key, c.session.IP) {
				keys = slices.Delete(keys, i, i+1)
				break
			}
		}
		info.P2P.Router = &RouterRuntimeInfo{
			Routers: keys,
			Version: c.p2p.router.MD5WithValue(),
		}
	}
	c.p2p.connections.Range(func(key, value any) bool {
		info.P2P.Connections = append(info.P2P.Connections, key.(*hub.Destination).Ip())
		return true
	})
	info.Metrics = c.transport.metrics
	info.Router = &RouterRuntimeInfo{}
	r := c.hub.Router()
	info.Router.Version = r.MD5()
	lm := c.hub.LatencyManager()

	// 手动刷新所有延迟统计（避免后台持续ping造成资源浪费）
	lm.RefreshAll()

	r.Range(func(addr string, val any) {
		if dst, ok := val.(*hub.Destination); ok {
			info.Router.Routers = append(info.Router.Routers, dst.Ip())
			info.Connections = append(info.Connections, &ConnectionInfo{
				Latency: lm.GetStats(dst.Ip()),
				Dst:     dst.Ip(),
				Type:    dst.Type(),
			})
		}
	})
	slices.SortFunc(info.Router.Routers, func(a, b string) int {
		return strings.Compare(a, b)
	})
	slices.SortFunc(info.Connections, func(a, b *ConnectionInfo) int {
		return strings.Compare(a.Dst, b.Dst)
	})
	return
}

func (c *Client) Stop(ctx context.Context) (err error) {
	g.Log().Infof(ctx, "stopping client...")
	c.state = StateStopped
	c.ReleaseAll()
	return
}

func (c *Client) Resume(ctx context.Context) (err error) {
	g.Log().Infof(ctx, "resuming client...")
	// set state to reconnecting to avoid reconnecting loop
	// break on stopped state
	c.state = StateReconnecting
	return c.Reconnect(context.Background())
}
