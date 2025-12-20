package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"go-vnet/common/config"
	"go-vnet/common/logger"
)

type (
	Server interface {
		Serve(ctx context.Context) error
		Close() (err error)
	}
	transportServer struct {
		ctx    context.Context
		sig    chan struct{}
		logger logger.Logger
		*ServerConfig
		Server
	}
)

func NewTransportServer(cfg *ServerConfig) Server {
	s := &transportServer{
		logger:       config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger),
		ServerConfig: cfg,
		sig:          make(chan struct{}),
	}
	switch cfg.Type {
	case TypeQuic:
		s.Server = newQuicServer(s)
	}
	return s
}

func (s *transportServer) Serve(ctx context.Context) error {
	if s.Server == nil {
		return fmt.Errorf("transportServer type not supported: %s", s.Type)
	}
	s.ctx = ctx
	return s.Server.Serve(ctx)
}

func (s *transportServer) handleFlowProxy(name string, dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	for {
		select {
		case <-s.sig:
			s.logger.Infof(s.ctx, "proxy tunnel closed: %s", name)
			return
		default:
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
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

func (s *transportServer) authAndRegisterRouter(ctx context.Context, conn any, receive func(ctx context.Context) ([]byte, error)) (payload map[string]any, src string, err error) {
	// Set a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func() {
		cancel()
	}()

	type remote interface {
		RemoteAddr() net.Addr
	}
	var rem = ""
	if r, ok := conn.(remote); ok {
		rem = r.RemoteAddr().String()
	}

	datagram, err := receive(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timeout while waiting for authentication data: %s", rem)
			return
		}
		return
	}

	payload, err = s.AuthorizedHandler.Handle(ctx, datagram)
	if err != nil {
		return
	}

	// register router
	src, ok := payload["address"].(string)
	if !ok {
		s.logger.Error(ctx, "auth connection data format error: missing or wrong type of address")
		return
	}

	if err = s.Router.Register(src, conn); err != nil {
		s.logger.Errorf(ctx, "register router error: %s", err.Error())
		return
	}

	s.logger.Infof(ctx, "handle connection: remote_addr=%s,route=%s", rem, src)
	return
}
