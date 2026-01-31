package server

import (
	"fmt"
	"sync"

	"github.com/quic-go/quic-go"

	"go-vnet/common/session"
	"go-vnet/vnet/server/network"
)

type (
	serverSession struct {
		*session.Session
		session.SendReceiveCloser
		ref     internalServer
		sig     chan struct{}
		cfg     *TransportConfig
		network *network.Network
		conn    any
		storage sync.Map
	}
)

const (
	sessionStorageKeyP2PPeer  = "p2p_peer"
	sessionStorageKeyLastPing = "last_ping"
)

func newServerSession(sr session.SendReceiveCloser, conn any) *serverSession {
	return &serverSession{
		SendReceiveCloser: sr,
		conn:              conn,
		storage:           sync.Map{},
		sig:               make(chan struct{}),
	}
}

func (s *serverSession) QuicConn() (c *quic.Conn, err error) {
	if s.Type != session.TypeQuic {
		err = fmt.Errorf("invalid session type: %s", s.Type)
		return
	}
	c = s.conn.(*quic.Conn)
	return
}

func (s *serverSession) Stop() {
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
