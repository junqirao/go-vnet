package server

import (
	"context"
	"io"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
)

type (
	Server struct {
		cfg     *Config
		logger  logger.Logger
		sig     chan struct{}
		auth    *auth.Server
		manager *Manager
	}
)

func NewServer(cfg *Config) *Server {
	s := &Server{
		cfg:     cfg,
		logger:  config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		sig:     make(chan struct{}),
		manager: NewManager(),
	}

	chainFunc := config.GetMappedConfig[[]auth.ServerAuthChainFunc](cfg, ConfigKeyAuthChainFunc, []auth.ServerAuthChainFunc{})
	s.auth = auth.NewServer(cfg.Auth, chainFunc)
	return s
}

func (s *Server) proxy(ctx context.Context, name string, dst io.Writer, src io.Reader) (written int64, err error) {
	buf := make([]byte, s.cfg.MTU*100)

	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		case <-s.sig:
			s.logger.Infof(ctx, "proxy tunnel closed: %s", name)
			return
		default:
		}

		// 优化：简化逻辑，移除不必要的错误检查
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if ew != nil {
				return written, ew
			}
			if nw > 0 {
				written += int64(nw)
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
