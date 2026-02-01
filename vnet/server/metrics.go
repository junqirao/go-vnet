package server

import (
	"time"
)

func (s *Server) backgroundUpdateMetricsLoop() {
	ticker := time.NewTicker(time.Second * 5)
	go func() {
		for {
			select {
			case <-s.sig:
				return
			case <-ticker.C:
				s.sessions.Range(func(key, value any) bool {
					ss := value.(*Session)
					ss.Metrics.UpdateAll()
					return true
				})
			}
		}
	}()
}
