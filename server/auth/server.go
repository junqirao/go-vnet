package auth

import (
	"context"
	"fmt"
)

type (
	Server struct {
		Encoder
		fns []ServerAuthChainFunc
	}
	ServerAuthChainFunc func(ctx context.Context, request map[string]any, resp map[string]any) (err error)
)

func NewServer(encoder Encoder, fns ...ServerAuthChainFunc) *Server {
	return &Server{Encoder: encoder, fns: fns}
}

func (s *Server) Auth(ctx context.Context, in []byte) (request, resp map[string]any, err error) {
	if s.Encoder == nil {
		err = fmt.Errorf("encoder not set")
		return
	}
	request, err = s.Decode(ctx, in)
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
