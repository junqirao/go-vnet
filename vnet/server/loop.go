package server

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	updateSessionMetricsInterval = time.Second * 1
)

func (s *Server) backgroundLoop() {
	metricsTicker := time.NewTicker(updateSessionMetricsInterval)
	go func() {
		for {
			select {
			case <-s.sig:
				return
			case <-metricsTicker.C:
				// session
				s.sessions.Range(func(key, value any) bool {
					ss := value.(*Session)
					s.updateSessionMetrics(ss)
					s.updateUsage(ss)
					return true
				})
				// network
				for _, network := range GetNetworkManager().GetAllNetworks() {
					network.Metrics.UpdateAll()
				}
			}
		}
	}()
}

func (s *Server) updateSessionMetrics(ss *Session) {
	ss.Metrics.UpdateAll()
	ss.network.Metrics.RxBytes.Add(ss.Metrics.RxBytes.Speed.Load())
	ss.network.Metrics.TxBytes.Add(ss.Metrics.TxBytes.Speed.Load())
	ss.network.Metrics.RxPackets.Add(ss.Metrics.RxPackets.Speed.Load())
	ss.network.Metrics.TxPackets.Add(ss.Metrics.TxPackets.Speed.Load())
}

func (s *Server) updateUsage(ss *Session) {
	_, err := ss.quota.CommitUsage()
	if err != nil {
		g.Log().Errorf(ss.Ctx, "commit usage error: %s", err.Error())
		return
	}
}
