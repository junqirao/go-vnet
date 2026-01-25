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
		cfg     *TransportConfig
		network *network.Network
		conn    any
		storage sync.Map
	}
)

func newServerSession(sr session.SendReceiveCloser, conn any) *serverSession {
	return &serverSession{
		SendReceiveCloser: sr,
		conn:              conn,
		storage:           sync.Map{},
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
