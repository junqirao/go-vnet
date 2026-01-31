package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/session"
)

type (
	Session struct {
		*session.Session
		session.SendReceiveCloser `json:"-"`
		CreatedAt                 time.Time `json:"created_at"`
		ref                       internalServer
		sig                       chan struct{}
		cfg                       *TransportConfig
		network                   *Network
		conn                      any
		storage                   sync.Map
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
