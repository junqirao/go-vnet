package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/metrics"
	"go-vnet/common/session"
)

type (
	Session struct {
		*session.Session
		session.SendReceiveCloser `json:"-"`

		ref     internalServer
		sig     chan struct{}
		cfg     *TransportConfig
		network *Network
		conn    any
		storage sync.Map

		ClientInfo ClientInfo                `json:"client_info"`
		Metrics    *metrics.TransportMetrics `json:"metrics"`
		CreatedAt  time.Time                 `json:"created_at"`
	}
	ClientInfo struct {
		Hostname any `json:"hostname"`
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
		storage:           sync.Map{},
		sig:               make(chan struct{}),
		CreatedAt:         time.Now(),
		Metrics:           metrics.NewTransportMetrics(),
	}
}

func (s *Session) QuicConn() (c *quic.Conn, err error) {
	if s.Type != session.TypeQuic {
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
