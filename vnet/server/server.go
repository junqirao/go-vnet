package server

import (
	"go-vnet/vnet/transport"
)

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Handshake() *transport.Session {

}
