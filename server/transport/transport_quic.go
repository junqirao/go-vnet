package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
)

type quicServer struct {
	*transportServer
}

func newQuicServer(ref *transportServer) *quicServer {
	return &quicServer{
		transportServer: ref,
	}
}

// Serve transportServer block
func (s *quicServer) Serve(ctx context.Context) (err error) {
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

	s.logger.Infof(ctx, "quic server running at %s", addr)

	defer func() {
		_ = listener.Close()
	}()

	for {
		select {
		case <-s.sig:
			s.logger.Info(ctx, "quic server closed")
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

func (s *quicServer) handleConnection(ctx context.Context, conn *quic.Conn) {
	_, src, err := s.authAndRegisterRouter(ctx, conn, conn.ReceiveDatagram, conn.SendDatagram)
	if err != nil {
		_ = conn.CloseWithError(403, "connection auth failed: "+err.Error())
		s.logger.Errorf(conn.Context(), "auth connection error: %s", err.Error())
		return
	}

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
		if _, err = s.handleFlowProxy(fmt.Sprintf("%s -> %s", src, dst), dst, stream, make([]byte, s.MTU)); err != nil {
			s.logger.Errorf(conn.Context(), "handle flow stopped. stream_id=%v error: %s", stream.StreamID(), err.Error())
			return
		}
	}

	for {
		select {
		case <-s.sig:
			s.logger.Info(ctx, "quic server closed")
			return
		default:
		}
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			s.logger.Errorf(conn.Context(), "accept stream error: %s", err.Error())
			return
		}
		go handle(stream)
	}
}

func (s *quicServer) Close() (err error) {
	close(s.sig)
	return
}
