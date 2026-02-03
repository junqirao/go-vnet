package server

import (
	"context"
	"sort"
	"sync"
	"time"

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
	}
	NetworkConfig struct {
		ID              string          `json:"id"`
		CIDR            string          `json:"cidr"`
		MTU             int             `json:"mtu"`
		RouterData      []byte          `json:"-"`
		AllocDeviceFunc AllocDeviceFunc `json:"-"`
	}
	AllocDeviceFunc func(ctx context.Context, n *Network, payload map[string]any) (dev *session.Device, err error)
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
	}
	return
}

func (n *Network) Router() *router.Router {
	return n.router
}

func (n *Network) AcquireDevice(ctx context.Context, sess *Session, request map[string]any) (dev *session.Device, err error) {
	dev = &session.Device{}
	if n.NetworkConfig.AllocDeviceFunc != nil {
		var d *session.Device
		if d, err = n.NetworkConfig.AllocDeviceFunc(ctx, n, request); err != nil {
			return
		}
		if d != nil {
			dev = d
		}
	}

	if dev.CIDR != "" {
		err = n.pool.AssignSpecific(dev.CIDR)
	} else {
		dev.CIDR, err = n.pool.AssignRandom()
	}
	if dev.MTU == 0 {
		dev.MTU = n.MTU
	}

	n.sessions.Store(dev.CIDR, sess)
	return
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

func (n *Network) Stop() {
	// stop all sessions
	n.sessions.Range(func(key, value any) bool {
		sess := value.(*Session)
		sess.Stop()
		return true
	})
}
