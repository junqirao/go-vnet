package server

import (
	"time"
)

const (
	updateSessionMetricsInterval = time.Second * 1
)

func (s *Server) backgroundUpdateMetricsLoop() {
	ticker := time.NewTicker(updateSessionMetricsInterval)
	go func() {
		for {
			select {
			case <-s.sig:
				return
			case <-ticker.C:
				// session
				s.sessions.Range(func(key, value any) bool {
					ss := value.(*Session)
					ss.Metrics.UpdateAll()
					ss.network.Metrics.RxBytes.Add(ss.Metrics.RxBytes.Speed.Load())
					ss.network.Metrics.TxBytes.Add(ss.Metrics.TxBytes.Speed.Load())
					ss.network.Metrics.RxPackets.Add(ss.Metrics.RxPackets.Speed.Load())
					ss.network.Metrics.TxPackets.Add(ss.Metrics.TxPackets.Speed.Load())
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
