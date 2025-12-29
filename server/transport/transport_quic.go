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
	info, err := s.authAndRegisterRouter(ctx, conn, conn.ReceiveDatagram, conn.SendDatagram)
	if err != nil {
		_ = conn.CloseWithError(403, "connection auth failed: "+err.Error())
		s.logger.Errorf(conn.Context(), "auth connection error: %s", err.Error())
		return
	}

	defer func() {
		_ = conn.CloseWithError(0, "connection closed")
		err := info.Network.ReleaseDevice(info.Device)
		if err != nil {
			s.logger.Errorf(conn.Context(), "release device error: %s", err.Error())
			return
		}
		s.logger.Infof(conn.Context(), "device released: %+v", info.Device)
		if err = info.Network.Router().Delete(conn.Context(), info.Src); err != nil {
			s.logger.Errorf(conn.Context(), "delete router error: %s", err.Error())
			return
		}
		s.logger.Infof(conn.Context(), "router deleted: %s", info.Src)
		s.logger.Infof(conn.Context(), "connection closed")
	}()

	for {
		select {
		case <-s.sig:
			s.logger.Info(ctx, "quic server closed")
			return
		default:
		}

		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			s.logger.Errorf(ctx, "accept stream error: %s", err.Error())
			return
		}
		// use first stream as default transport connection
		if info.GetRWC() == nil {
			info.SetRWC(stream)
			s.logger.Infof(ctx, "default transport connection set: %v", stream.StreamID())
			continue
		}

		// clone
		info := info.Clone()
		info.SetRWC(stream)
		go s.handleConn(conn.Context(), stream, info)
	}
}

func (s *quicServer) Close() (err error) {
	close(s.sig)
	return
}
