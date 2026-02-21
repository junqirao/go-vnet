package server

import (
	"time"
)

const (
	updateSessionMetricsInterval = time.Second * 1
	activeThresholdSpeed         = 1024 // 1kb/s
)

func (s *Server) backgroundLoop() {
	metricsTicker := time.NewTicker(updateSessionMetricsInterval)
	updateUsageTicker := time.NewTicker(time.Minute)
	pushUsageToActiveSessionsTicker := time.NewTicker(time.Minute * 5)
	go func() {
		for {
			select {
			case <-s.sig:
				return
			case <-pushUsageToActiveSessionsTicker.C:
				s.pushQuotaUsageUpdateEvent()
			case <-updateUsageTicker.C:
				// session
				s.sessions.Range(func(key, value any) bool {
					ss := value.(*Session)
					s.updateUsage(ss)
					return true
				})
			case now := <-metricsTicker.C:
				// session
				s.sessions.Range(func(key, value any) bool {
					ss := value.(*Session)
					s.updateSessionMetrics(ss)
					rx := ss.Metrics.RxBytes.Speed.Load()
					tx := ss.Metrics.TxBytes.Speed.Load()
					if rx+tx > activeThresholdSpeed {
						ss.LastActive = now
					}
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
	if ss.quota == nil {
		return
	}
	ss.quota.CommitUsage()
}
