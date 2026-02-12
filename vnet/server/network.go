package server

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/gogf/gf/v2/util/grand"

	"go-vnet/common/addresses"
	"go-vnet/common/flow"
	"go-vnet/common/metrics"
	"go-vnet/common/router"
	"go-vnet/common/session"
)

type (
	Network struct {
		NetworkConfig `json:"config"`
		Metrics       *metrics.TransportMetrics `json:"metrics"`
		sessions      sync.Map                  // cidr : *Session
		router        *router.Router
		pool          *addresses.IPAllocator
		control       *flow.Control
		StartedAt     time.Time
		key           string
	}
	NetworkConfig struct {
		ID                   string `json:"id"`
		CIDR                 string `json:"cidr"`
		MTU                  int    `json:"mtu"`
		DataTrafficQuota     int64  `json:"data_traffic_quota"`
		DataTrafficQuotaType string `json:"data_traffic_quota_type"`
		RouterData           []byte `json:"-"`
	}
	AllocDeviceFunc func(ctx context.Context, payload map[string]any) (dev *session.Device, err error)
)

func NewNetwork(cfg *NetworkConfig) (n *Network, err error) {
	allocator, err := addresses.NewIPAllocator(cfg.CIDR)
	if err != nil {
		return
	}
	n = &Network{
		NetworkConfig: *cfg,
		Metrics:       metrics.NewTransportMetrics(),
		router:        router.NewRouter(),
		pool:          allocator,
		StartedAt:     time.Now(),
		key:           grand.S(32),
	}
	return
}

func (n *Network) Router() *router.Router {
	return n.router
}

func (n *Network) ReleaseDevice(cidr string) (err error) {
	n.sessions.Delete(cidr)
	return n.pool.ReleaseIP(cidr)
}

func (n *Network) Control() *flow.Control {
	return n.control
}

func (n *Network) SetControl(control *flow.Control) {
	n.control = control
}

func (n *Network) ListSessions() (sessions []*Session) {
	n.sessions.Range(func(key, value any) bool {
		sessions = append(sessions, value.(*Session))
		return true
	})
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.Unix() < sessions[j].CreatedAt.Unix()
	})
	return
}

func (n *Network) SessionById(id string) (s *Session, ok bool) {
	n.sessions.Range(func(key, value any) bool {
		if v := value.(*Session); v.SessionId == id {
			s = v
			ok = true
			return false
		}
		return true
	})
	return
}

func (n *Network) Stop() {
	// stop all sessions
	n.sessions.Range(func(key, value any) bool {
		sess := value.(*Session)
		sess.Stop()
		return true
	})
}

func (n *Network) RegisterSession(cidr string, s *Session) {
	n.sessions.Store(cidr, s)
}

func (n *Network) AddressPool() *addresses.IPAllocator {
	return n.pool
}
