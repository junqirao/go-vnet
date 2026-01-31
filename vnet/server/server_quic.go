package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	"go-vnet/common/session"
	tt "go-vnet/common/tls"
)

type (
	quicServer struct {
		*Server
		cfg      *TransportConfig
		listener *quic.Listener
	}
)

func newQuicServer(s *Server) internalServer {
	return &quicServer{
		Server: s,
	}
}

func (s *quicServer) Setup(ctx context.Context, cfg *TransportConfig) (err error) {
	s.cfg = cfg
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](s.cfg, ConfigKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	quicConfig := config.GetMappedConfig[*quic.Config](s.cfg, ConfigKeyQuicConfig, config.DefaultQuicConfig)

	// s.logger.Infof(ctx, "use quic config %+v", quicConfig)

	// listen
	addr := fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port)
	s.listener, err = quic.ListenAddr(addr, tlsConfig, quicConfig)
	return
}

func (s *quicServer) Accept(ctx context.Context) (ss *Session, err error) {
	c, err := s.listener.Accept(ctx)
	if err != nil {
		return
	}
	ss = newServerSession(session.SendReceiverFromQuicConn(c), c)
	return
}

func (s *quicServer) Close() error {
	return s.listener.Close()
}

func (s *quicServer) AcceptTransport(session *Session) (rwc io.ReadWriteCloser, err error) {
	conn, err := session.QuicConn()
	if err != nil {
		return nil, err
	}
	rwc, err = conn.AcceptStream(session.Ctx)
	return
}

func (s *quicServer) GetDstTransportWriter(src *Session, dst *Session) (rwc io.ReadWriteCloser, err error) {
	conn, err := dst.QuicConn()
	if err != nil {
		s.logger.Errorf(src.Ctx, "internal error: %s", err.Error())
		return
	}

	dstStream, err := conn.OpenStreamSync(src.Ctx)
	if err != nil {
		s.logger.Errorf(src.Ctx, "open stream error: dst=%v", dst.SessionId)
		return
	}
	s.logger.Infof(src.Ctx, "open stream success: dst=%v id=%d", dst.SessionId, dstStream.StreamID().StreamNum())
	rwc = dstStream
	return
}
