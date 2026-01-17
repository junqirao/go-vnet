package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
	"go-vnet/vnet/session"
)

type (
	quicServer struct {
		*Server
		listener *quic.Listener
	}
)

func newQuicServer(s *Server) internalServer {
	return &quicServer{
		Server: s,
	}
}

func (s *quicServer) Run(ctx context.Context) (err error) {
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](s.cfg, ConfigKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	quicConfig := config.GetMappedConfig[*quic.Config](s.cfg, ConfigKeyQuicConfig, config.DefaultQuicConfig)

	s.logger.Infof(ctx, "use quic config %+v", quicConfig)

	// listen
	addr := fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port)
	s.listener, err = quic.ListenAddr(addr, tlsConfig, quicConfig)
	return
}

func (s *quicServer) Accept(ctx context.Context) (sr session.SendReceiveCloser, conn any, err error) {
	c, err := s.listener.Accept(ctx)
	if err != nil {
		return
	}
	conn = c
	sr = session.SendReceiverFromQuicConn(c)
	return
}

func (s *quicServer) Close() error {
	return s.listener.Close()
}

func (s *quicServer) HandleProxy(ctx context.Context, session *serverSession) (err error) {
	conn, ok := session.conn.(*quic.Conn)
	if !ok {
		return fmt.Errorf("invalid connection type: %T", session.conn)
	}
	for {
		select {
		case <-s.sig:
			return
		case <-ctx.Done():
			return
		default:
			stream, err := conn.AcceptStream(ctx)
			if err != nil {
				s.logger.Errorf(ctx, "accept stream error: %s", err.Error())
				return err
			}
			go s.handleStreamProxy(ctx, session, stream)
		}
	}
}

func (s *quicServer) handleStreamProxy(ctx context.Context, sess *serverSession, stream *quic.Stream) {
	// get dst ip by first packet, 10s timeout
	var (
		buf   = make([]byte, 15)
		ch    = make(chan []byte)
		first []byte
	)

	go func() {
		n, err := stream.Read(buf)
		if err != nil {
			return
		}
		ch <- buf[:n]
	}()

	timer := time.NewTimer(time.Second * 10)
	select {
	case <-timer.C:
		s.logger.Errorf(ctx, "read first pkg timeout. src=%v", sess.IP)
		return
	case first = <-ch:
	}

	var (
		dst = string(first)
		src = sess.IP
	)

	v, ok := sess.network.Router().RouteString(dst)
	if !ok {
		s.logger.Errorf(ctx, "route not found: dst=%v", dst)
		_ = stream.Close()
		return
	}

	// send ack (byte 1) to client
	_, _ = stream.Write([]byte{1})

	s.logger.Infof(ctx, "handle stream proxy: %v->%v", src, dst)

	dstSession, ok := v.(*serverSession)
	if !ok {
		s.logger.Errorf(ctx, "error session type: dst=%v", dst)
		return
	}

	conn := dstSession.conn.(*quic.Conn)
	dstStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		s.logger.Errorf(ctx, "open stream error: dst=%v", dst)
		return
	}

	var (
		written int64
		name    = fmt.Sprintf("%s->%s", src, dst)
	)

	defer func() {
		_ = dstStream.Close()
		_ = stream.Close()
		s.logger.Infof(ctx, "proxy stopped: %s,written=%v", name, written)
	}()

	written, err = s.proxy(ctx, name, dstStream, stream)
	if err != nil {
		s.logger.Errorf(ctx, "proxy error: %s ,err=%v", name, err.Error())
		return
	}
}
