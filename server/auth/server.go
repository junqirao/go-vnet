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

func NewServer(cfg Config, fns []ServerAuthChainFunc, encoder ...Encoder) *Server {
	s := &Server{fns: fns}
	if len(encoder) > 0 {
		s.Encoder = encoder[0]
		return s
	}
	switch cfg.Type {
	case TypeRSA:
		var opts []RSAEncoderOption
		if cfg.PublicKey != "" {
			opts = append(opts, WithPublicKey(cfg.PublicKey))
		}
		if cfg.PrivateKey != "" {
			opts = append(opts, WithPrivateKey(cfg.PrivateKey))
		}
		s.Encoder = NewRsaEncoder(opts...)
	default:
		s.Encoder = NewSimplePasswordEncoder(cfg.Password)
	}
	return s
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
