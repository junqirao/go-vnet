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

	"go-vnet/server/network"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
)

type (
	quicServer struct {
		*Server

		sessions sync.Map // src : *quicSession
	}
	quicSession struct {
		*Session
		conn *quic.Conn
	}
	quicSendReceiver struct {
		*quic.Conn
	}
)

func (q quicSendReceiver) Send(data []byte) (err error) {
	return q.SendDatagram(data)
}

func (q quicSendReceiver) Receive(ctx context.Context) (data []byte, err error) {
	return q.ReceiveDatagram(ctx)
}

func newQuicServer(s *Server) *quicServer {
	return &quicServer{
		Server: s,
	}
}

func (s *quicServer) Serve(ctx context.Context) (err error) {
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](s.cfg, ConfigKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	quicConfig := config.GetMappedConfig[*quic.Config](s.cfg, ConfigKeyQuicConfig, config.DefaultQuicConfig)

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

			// accept datagram for manager
			go s.handleDatagramLoop(ctx, session)

			// accept stream
			go s.acceptStreamLoop(ctx, session)
		}
	}
}

func (s *quicServer) registerConn(ctx context.Context, conn *quic.Conn) (session *quicSession, err error) {
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

	session = &quicSession{
		Session: &Session{
			SendReceiver:     &quicSendReceiver{conn},
			Type:             TypeQuic,
			Id:               "todo-session-id",
			Network:          nwk,
			DispatchedDevice: dev,
			IP:               ip.To4().String(),
		},
		conn: conn,
	}

	// register router
	if err = nwk.Router().Register(ctx, fmt.Sprintf("%s/32", session.IP), session); err != nil {
		s.logger.Errorf(ctx, "register router error: %s", err.Error())
		s.closeWithError(ctx, conn, err, 400)
		return
	}
	// register session
	s.sessions.Store(session.IP, session)
	// all these registration will be unregistered in acceptStreamLoop
	// when the connection is closed (can not accept new stream)
	s.logger.Infof(ctx, "handle connection: remote_addr=%s,route=%s", remote, session.IP)
	return
}

func (s *quicServer) acceptStreamLoop(ctx context.Context, session *quicSession) {
	defer func() {
		// release device
		err := session.Network.ReleaseDevice(session.DispatchedDevice)
		if err != nil {
			s.logger.Errorf(ctx, "release device error: %s", err.Error())
		}
		// unregister session
		s.sessions.Delete(session.IP)
		// unregister router
		if err := session.Network.Router().Delete(ctx, fmt.Sprintf("%s/32", session.IP)); err != nil {
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
				return
			}
			go s.handleStreamProxy(ctx, session, stream)
		}
	}
}

func (s *quicServer) handleStreamProxy(ctx context.Context, session *quicSession, stream *quic.Stream) {
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
		s.logger.Errorf(ctx, "read first pkg timeout. src=%v", session.IP)
		return
	case first = <-ch:
	}

	var (
		dst = string(first)
		src = session.IP
	)

	v, ok := session.Network.Router().Route(dst)
	if !ok {
		s.logger.Errorf(ctx, "route not found: dst=%v", dst)
		_ = stream.Close()
		return
	}

	// send ack (byte 1) to client
	_, _ = stream.Write([]byte{1})

	s.logger.Infof(ctx, "handle stream proxy: %v->%v", src, dst)

	dstSession, ok := v.(*quicSession)
	if !ok {
		s.logger.Errorf(ctx, "error session type: dst=%v", dst)
		return
	}
	dstStream, err := dstSession.conn.OpenStreamSync(ctx)
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

func (s *quicServer) handleDatagramLoop(ctx context.Context, session *quicSession) {
	var (
		err      error
		datagram []byte
	)

	defer func() {
		s.logger.Infof(ctx, "handle datagram loop stopped: %s, reason=%s", session.IP, err.Error())
	}()

	for {
		select {
		case <-s.sig:
			err = errors.New("server closed")
			return
		case <-ctx.Done():
			err = ctx.Err()
			return
		default:
			datagram, err = session.conn.ReceiveDatagram(ctx)
			if err != nil {
				return
			}
			s.manager.PushEvent(session.Session, datagram)
		}
	}
}

func (s *quicServer) closeWithError(ctx context.Context, conn *quic.Conn, err error, code uint64) {
	if err == nil {
		return
	}
	s.logger.Errorf(ctx, "connection closed with error: %s", err.Error())
	_ = conn.CloseWithError(quic.ApplicationErrorCode(code), err.Error())
}
