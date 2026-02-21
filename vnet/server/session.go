package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/quic-go/quic-go"

	"go-vnet/common/metrics"
	"go-vnet/common/protocol"
	"go-vnet/common/quota"
	"go-vnet/common/rate"
	"go-vnet/common/session"
)

type (
	Session struct {
		*session.Session
		session.SendReceiveCloser `json:"-"`

		ref              internalServer
		sig              chan struct{}
		cfg              *TransportConfig
		network          *Network
		quota            *quota.Quota
		bandwidth        int64
		bandwidthLimiter *rate.SmoothLimiter
		conn             any
		storage          sync.Map

		ClientInfo ClientInfo                `json:"client_info"`
		Metrics    *metrics.TransportMetrics `json:"metrics"`
		Proxying   sync.Map                  `json:"-"`
		CreatedAt  time.Time                 `json:"created_at"`
		LastActive time.Time                 `json:"last_active"`
	}
	ClientInfo struct {
		Hostname any `json:"hostname"`
		Encrypt  any `json:"encrypt"`
		Compress any `json:"compress"`
		P2P      any `json:"p2p"`
	}
)

const (
	sessionStorageKeyP2PPeer  = "p2p_peer"
	sessionStorageKeyLastPing = "last_ping"
)

func newServerSession(sr session.SendReceiveCloser, conn any) *Session {
	return &Session{
		SendReceiveCloser: sr,
		conn:              conn,
		sig:               make(chan struct{}),
		storage:           sync.Map{},
		Proxying:          sync.Map{},
		CreatedAt:         time.Now(),
		Metrics:           metrics.NewTransportMetrics(),
	}
}

func (s *Session) QuicConn() (c *quic.Conn, err error) {
	if s.Type != protocol.TransportTypeQuic {
		err = fmt.Errorf("invalid session type: %s", s.Type)
		return
	}
	c = s.conn.(*quic.Conn)
	return
}

func (s *Session) Stop() {
	select {
	case _, ok := <-s.sig:
		if !ok {
			return
		}
	default:
		close(s.sig)
	}
	return
}

func (s *Session) Network() *Network {
	return s.network
}

func (s *Session) SetNetwork(n *Network) {
	s.network = n
}

func (s *Session) Release(e error) int64 {
	// submit usage
	usage := s.quota.CommitUsage()
	// release device
	err := s.network.ReleaseDevice(s.DispatchedDevice.CIDR)
	if err != nil {
		g.Log().Errorf(s.Ctx, "release device error: %s", err.Error())
	}
	// unregister router
	s.network.Router().UnRegister(fmt.Sprintf("%s/32", s.IP))
	// close connection
	s.CloseWithError(e)
	return usage
}
