package server

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/common/quota"
)

const (
	EventNameQuotaUsageUpdate = "quota_usage_update"
)

func (s *Server) pushQuotaUsageUpdateEvent() {
	var (
		now      = time.Now()
		start    = now.Add(-5 * time.Minute)
		sessions []*Session
	)

	// current sessions
	s.sessions.Range(func(key, value any) bool {
		sess := value.(*Session)
		// has quota
		if sess.quota.GetMaxLimit() != quota.ValueNoLimit {
			// only push event if session is activated in 5 minutes
			if !sess.LastActive.After(start) {
				// not activated
				return true
			}
			sessions = append(sessions, sess)
		}
		return true
	})

	// push event
	for _, session := range sessions {
		usage, err := session.quota.Adaptor().LoadUsage(session.Ctx)
		if err != nil {
			g.Log().Errorf(session.Ctx, "load quota usage error: %s", err.Error())
			continue
		}
		g.Log().Infof(session.Ctx, "push quota usage update event: %d", usage)
		if err := s.manager.PushEvent(session, EventNameQuotaUsageUpdate, usage); err != nil {
			g.Log().Errorf(session.Ctx, "push quota usage update event error: %s", err.Error())
		}
	}
}
