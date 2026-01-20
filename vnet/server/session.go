package server

import (
	"fmt"

	"github.com/quic-go/quic-go"

	"go-vnet/common/session"
	"go-vnet/vnet/server/network"
)

type (
	serverSession struct {
		*session.Session
		session.SendReceiveCloser
		network *network.Network
		conn    any
	}
)

func (s *serverSession) QuicConn() (c *quic.Conn, err error) {
	if s.Type != session.TypeQuic {
		err = fmt.Errorf("invalid session type: %s", s.Type)
		return
	}
	c = s.conn.(*quic.Conn)
	return
}
