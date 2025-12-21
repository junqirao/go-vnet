package auth

import (
	"context"
	"fmt"
)

type (
	Server struct {
		encoder Encoder
		fns     []ServerAuthChainFunc
	}
	ServerAuthChainFunc func(ctx context.Context, request map[string]any, resp map[string]any) (err error)
)

func NewServer(encoder Encoder, fns ...ServerAuthChainFunc) *Server {
	return &Server{encoder: encoder, fns: fns}
}

func (s *Server) Auth(ctx context.Context, in []byte) (resp map[string]any, err error) {
	if s.encoder == nil {
		err = fmt.Errorf("encoder not set")
		return
	}
	request, err := s.encoder.Decode(ctx, in)
	if err != nil {
		return
	}
	resp = make(map[string]any)
	for _, fn := range s.fns {
		if err = fn(ctx, request, resp); err != nil {
			return
		}
	}
	return
}
