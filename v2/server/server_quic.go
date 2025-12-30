package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	tt "go-vnet/common/tls"
	"go-vnet/server/network"
)

type (
	QuicServer struct {
		// generic
		cfg     *Config
		logger  logger.Logger
		sig     chan struct{}
		auth    *auth.Server
		manager *Manager

		sessions sync.Map // src : *QuicSession
	}
	QuicSession struct {
		*Session
		conn *quic.Conn
	}
)

func NewQuicServer(cfg *Config) *QuicServer {
	s := &QuicServer{
		cfg:     cfg,
		logger:  config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		sig:     make(chan struct{}),
		manager: NewManager(),
	}

	chainFunc := config.GetMappedConfig[[]auth.ServerAuthChainFunc](cfg, ConfigKeyAuthChainFunc, []auth.ServerAuthChainFunc{})
	s.auth = auth.NewServer(cfg.Auth, chainFunc)
	return s
}

func (s *QuicServer) Serve(ctx context.Context) (err error) {
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](s.cfg, ConfigKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	quicConfig := config.GetMappedConfig[*quic.Config](s.cfg, ConfigKeyQuicConfig)

	s.logger.Infof(ctx, "use quic config %+v", quicConfig)

	// listen
	addr := fmt.Sprintf("%s:%d", s.cfg.Address, s.cfg.Port)
	listener, err := quic.ListenAddr(addr, tlsConfig, quicConfig)
	if err != nil {
		return
	}

	s.logger.Infof(ctx, "quic server running at %s", addr)

	defer func() {
		_ = listener.Close()
	}()

	go func() {
		_ = s.manager.ProcessFuncCallLoop(ctx)
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

			// register
			session, err := s.registerConn(ctx, conn)
			if err != nil {
				s.logger.Errorf(ctx, "failed to register connection: %s", err.Error())
				continue
			}

			// accept stream
			go s.acceptStreamLoop(ctx, session)
		}
	}
}

func (s *QuicServer) registerConn(ctx context.Context, conn *quic.Conn) (session *QuicSession, err error) {
	// Set a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func() {
		cancel()
	}()

	remote := conn.RemoteAddr().String()

	datagram, err := conn.ReceiveDatagram(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timeout while waiting for authentication data: %s", remote)
			return
		}
		return
	}

	request, resp, err := s.auth.Auth(ctx, datagram)
	if err != nil {
		s.closeWithError(ctx, conn, err, 401)
		return
	}

	defer func() {
		if err != nil {
			resp["error"] = err.Error()
		}
		// send to client
		bs, err := s.auth.Encode(ctx, resp)
		if err != nil {
			s.logger.Errorf(ctx, "encode auth response error: %s", err.Error())
			return
		}

		_ = conn.SendDatagram(bs)
	}()

	// 通过network_id获取network
	networkId, ok := request["network_id"].(string)
	if !ok {
		err = fmt.Errorf("network_id field from request not found")
		s.closeWithError(ctx, conn, err, 400)
		return
	}

	nwk, ok := network.GetManager().GetNetwork(networkId)
	if !ok {
		err = fmt.Errorf("network not found: id=%s", networkId)
		s.closeWithError(ctx, conn, err, 404)
		return
	}

	dev, err := nwk.AcquireDevice(ctx, request)
	if err != nil {
		err = fmt.Errorf("acquire device error: %s", err.Error())
		s.closeWithError(ctx, conn, err, 400)
		return
	}
	s.logger.Infof(ctx, "dispatch device: id=%v cidr=%v", dev.Id, dev.CIDR)

	resp["device"] = dev

	// register router
	ip, _, _ := net.ParseCIDR(dev.CIDR)

	session = &QuicSession{
		Session: &Session{
			SendReceiver:     &quicSendReceiver{conn},
			Id:               "todo-session-id",
			Network:          nwk,
			DispatchedDevice: dev,
			IP:               fmt.Sprintf("%s/32", ip.To4().String()),
		},
		conn: conn,
	}

	// register router
	if err = nwk.Router().Register(ctx, session.IP, session); err != nil {
		s.logger.Errorf(ctx, "register router error: %s", err.Error())
		s.closeWithError(ctx, conn, err, 400)
		return
	}
	// register session
	s.sessions.Store(session.IP, session)
	// register manager connection
	s.manager.Register(session.IP, session.Session)
	// all these registration will be unregistered in acceptStreamLoop
	// when the connection is closed (can not accept new stream)
	s.logger.Infof(ctx, "handle connection: remote_addr=%s,route=%s", remote, session.IP)
	return
}

func (s *QuicServer) acceptStreamLoop(ctx context.Context, session *QuicSession) {
	defer func() {
		// release device
		err := session.Network.ReleaseDevice(session.DispatchedDevice)
		if err != nil {
			s.logger.Errorf(ctx, "release device error: %s", err.Error())
		}
		// unregister session
		s.sessions.Delete(session.IP)
		// unregister router
		if err := session.Network.Router().Delete(ctx, session.IP); err != nil {
			s.logger.Errorf(ctx, "unregister router error: %s", err.Error())
		}
		// delete manager connection
		s.manager.DeleteSession(session.IP)
		// close connection
		_ = session.conn.CloseWithError(0, "connection closed")
	}()

	for {
		select {
		case <-s.sig:
			return
		case <-ctx.Done():
			return
		default:
			stream, err := session.conn.AcceptStream(ctx)
			if err != nil {
				s.logger.Errorf(ctx, "accept stream error: %s", err.Error())
				return
			}
			go s.handleStreamProxy(ctx, session, stream)
		}
	}
}

func (s *QuicServer) handleStreamProxy(ctx context.Context, session *QuicSession, stream *quic.Stream) {

}

func (s *QuicServer) closeWithError(ctx context.Context, conn *quic.Conn, err error, code uint64) {
	if err == nil {
		return
	}
	s.logger.Errorf(ctx, "connection closed with error: %s", err.Error())
	_ = conn.CloseWithError(quic.ApplicationErrorCode(code), err.Error())
}
