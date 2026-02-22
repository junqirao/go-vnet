package server

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"go-vnet/common/quota"
)

const (
	EventNameQuotaUsageUpdate = "quota_usage_update"
	EventNameP2PPeerUpdate    = "p2p_peer_update"
	EventNameP2PPeerDelete    = "p2p_peer_delete"
	EventNameRouterUpdate     = "router_update"
	EventNameRouterDelete     = "router_delete"
)

type (
	P2PPeerEventData struct {
		Ip   string `json:"ip"`
		Peer string `json:"peer,omitempty"`
	}
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
		// ignore push error
		s.manager.PushEventAsync(session, EventNameQuotaUsageUpdate, usage)
	}
}

func (s *Server) BroadcastPeer(ctx context.Context, eventName string, from *Session, peer string) {
	push := 0
	total := 0
	errorFunc := func(session *Session, event string, data any, err error) {
		g.Log().Errorf(ctx, "broadcast %s peer to %s error: %s", from.IP, session.IP, err.Error())
		// dont retry, client will fetch
	}
	s.sessions.Range(func(key, value any) bool {
		sess := value.(*Session)
		total++
		// ignore self and non-p2p session
		if sess.SessionId == from.SessionId || !gconv.Bool(sess.ClientInfo.P2P) {
			return true
		}
		push++
		data := P2PPeerEventData{
			Peer: peer,
			Ip:   sess.IP,
		}
		s.manager.PushEventAsync(sess, eventName, data, errorFunc)
		return true
	})

	g.Log().Infof(ctx, "broadcast peer event: %s, push: %d/%d", eventName, push, total)
}

func (s *Server) BroadcastRouter(ctx context.Context, eventName string, from *Session, router string) {
	push := 0
	total := 0
	errorFunc := func(session *Session, event string, data any, err error) {
		g.Log().Errorf(ctx, "broadcast %s router to %s error: %s", from.IP, session.IP, err.Error())
		// dont retry, client will fetch
	}
	s.sessions.Range(func(key, value any) bool {
		sess := value.(*Session)
		total++
		// ignore self
		if sess.SessionId == from.SessionId {
			return true
		}
		push++
		s.manager.PushEventAsync(sess, eventName, router, errorFunc)
		return true
	})

	g.Log().Infof(ctx, "broadcast peer event: %s, push: %d/%d", eventName, push, total)
}
