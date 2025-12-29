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
		cfg    *Config
		logger logger.Logger
		sig    chan struct{}
		auth   *auth.Server

		sessions sync.Map // src : *QuicSession
	}
	QuicSession struct {
		Session
		conn *quic.Conn
	}
	quicSendReceiver struct {
		*quic.Conn
	}
)

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
		s.logger.Error(ctx, err.Error())
		return
	}

	nwk, ok := network.GetManager().GetNetwork(networkId)
	if !ok {
		err = fmt.Errorf("network not found: id=%s", networkId)
		s.logger.Error(ctx, err.Error())
		return
	}

	dev, err := nwk.AcquireDevice(ctx, request)
	if err != nil {
		err = fmt.Errorf("acquire device error: %s", err.Error())
		s.logger.Errorf(ctx, err.Error())
		return
	}
	s.logger.Infof(ctx, "dispatch device: id=%v cidr=%v", dev.Id, dev.CIDR)

	resp["device"] = dev

	// register router
	ip, _, _ := net.ParseCIDR(dev.CIDR)

	session = &QuicSession{
		Session: Session{
			SendReceiver:     &quicSendReceiver{conn},
			Id:               "todo-session-id",
			Network:          nwk,
			DispatchedDevice: dev,
			IP:               fmt.Sprintf("%s/32", ip.To4().String()),
		},
		conn: conn,
	}

	if err = nwk.Router().Register(ctx, session.IP, session); err != nil {
		s.logger.Errorf(ctx, "register router error: %s", err.Error())
		return
	}

	s.sessions.Store(session.IP, session)
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
				continue
			}
			go s.handleStreamProxy(ctx, session, stream)
		}
	}
}

func (s *QuicServer) handleStreamProxy(ctx context.Context, session *QuicSession, stream *quic.Stream) {

}

func (q quicSendReceiver) Send(data []byte) (err error) {
	return q.SendDatagram(data)
}

func (q quicSendReceiver) Receive(ctx context.Context) (data []byte, err error) {
	return q.ReceiveDatagram(ctx)
}
