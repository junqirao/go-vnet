package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	"go-vnet/common/logger"
	tt "go-vnet/common/tls"
)

type QuicServer struct {
	logger logger.Logger
	ctx    context.Context
	*Config
	sig chan struct{}
}

func NewQuicServer(cfg *Config) *QuicServer {
	return &QuicServer{
		logger: config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger),
		Config: cfg,
		sig:    make(chan struct{}),
	}
}

// Serve server block
func (s *QuicServer) Serve(ctx context.Context) (err error) {
	s.ctx = ctx
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](s, configKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	quicConfig := config.GetMappedConfig[*quic.Config](s, configKeyQuicConfig)

	s.logger.Infof(ctx, "use quic config %+v", quicConfig)

	// listen
	addr := fmt.Sprintf("%s:%d", s.Address, s.Port)
	listener, err := quic.ListenAddr(addr, tlsConfig, quicConfig)
	if err != nil {
		return
	}

	s.logger.Infof(s.ctx, "quic server running at %s", addr)

	defer func() {
		_ = listener.Close()
	}()

	for {
		select {
		case <-s.sig:
			s.logger.Info(s.ctx, "quic server closed")
			return
		default:
			ctx := context.Background()
			conn, err := listener.Accept(ctx)
			if err != nil {
				s.logger.Errorf(ctx, "accept connection error: %s", err.Error())
				continue
			}
			go s.handleConnection(ctx, conn)
		}
	}
}

func (s *QuicServer) handleConnection(ctx context.Context, conn *quic.Conn) {
	payload, err := s.authConnection(conn)
	if err != nil {
		s.logger.Errorf(conn.Context(), "auth connection error: %s", err.Error())
		return
	}

	// register router
	dst, ok := payload["address"].(string)
	if !ok {
		s.logger.Error(conn.Context(), "auth connection data format error: missing or wrong type of address")
		return
	}
	err = s.Router.Register(dst, conn)
	if err != nil {
		s.logger.Errorf(conn.Context(), "register router error: %s", err.Error())
		return
	}

	s.logger.Infof(ctx, "handle connection: remote_addr=%s,route=%s", conn.RemoteAddr().String(), dst)

	defer func() {
		_ = conn.CloseWithError(0, "connection closed")
	}()

	handle := func(stream *quic.Stream) {
		defer func() {
			_ = stream.Close()
		}()
		// route
		dst, err := s.Route(ctx, stream)
		if err != nil {
			s.logger.Errorf(conn.Context(), "route error: %s", err.Error())
			return
		}

		s.logger.Infof(ctx, "handle flow start. stream_id=%v", stream.StreamID())

		// block and redirect flow to s.dst
		if _, err = s.handleFlowProxy(dst, stream, make([]byte, s.MTU)); err != nil {
			s.logger.Errorf(conn.Context(), "handle flow stopped. stream_id=%v error: %s", stream.StreamID(), err.Error())
			return
		}
	}

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			s.logger.Errorf(conn.Context(), "accept stream error: %s", err.Error())
			return
		}
		go handle(stream)
	}
}

func (s *QuicServer) handleFlowProxy(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	for {
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

func (s *QuicServer) authConnection(conn *quic.Conn) (payload map[string]any, err error) {
	// Set a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(conn.Context(), 10*time.Second)
	defer func() {
		cancel()
		if err != nil {
			_ = conn.CloseWithError(1000, "auth connection error: "+err.Error())
		}
	}()

	datagram, err := conn.ReceiveDatagram(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timeout while waiting for authentication datagram: %s", conn.RemoteAddr().String())
			return
		}
		return
	}

	return s.AuthorizedHandler.Handle(conn.Context(), datagram)
}

func (s *QuicServer) Close() (err error) {
	close(s.sig)
	return
}
