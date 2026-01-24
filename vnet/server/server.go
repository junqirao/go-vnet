package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/common/protocol"
	"go-vnet/common/session"
	"go-vnet/vnet/server/consts"
	"go-vnet/vnet/server/network"
)

var (
	ErrUnauthorized     = session.NewError("unauthorized", 401)
	ErrInvalidParameter = session.NewError("invalid parameter", 400)
	ErrResourceNotFound = session.NewError("resource not found", 404)
	ErrResourceError    = session.NewError("resource error", 406)
)

type (
	Server struct {
		internals sync.Map // name : internalServer
		cfg       *Config
		logger    logger.Logger
		sig       chan struct{}
		auth      *auth.Server
		manager   *Manager
		sessions  sync.Map // src : *serverSession
	}
	internalServer interface {
		io.Closer
		Setup(ctx context.Context, cfg *TransportConfig) (err error)
		Accept(ctx context.Context) (ss *serverSession, err error)
		AcceptTransport(session *serverSession) (rwc io.ReadWriteCloser, err error)
		GetDstTransportWriter(src *serverSession, dst *serverSession) (rwc io.ReadWriteCloser, err error)
	}
)

func NewServer(cfg *Config) *Server {
	s := &Server{
		cfg:     cfg,
		logger:  config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		sig:     make(chan struct{}),
		manager: NewManager(),
	}

	chainFunc := config.GetMappedConfig[[]auth.ServerAuthChainFunc](cfg,
		ConfigKeyAuthChainFunc, []auth.ServerAuthChainFunc{})
	s.auth = auth.NewServer(cfg.Auth, chainFunc)
	return s
}

func (s *Server) Serve(ctx context.Context) (err error) {
	go func() {
		_ = s.manager.ProcessFuncCallLoop(ctx)
	}()
	defer func() {
		_ = s.manager.Close()
	}()

	for _, server := range s.cfg.Servers {
		go func(cfg *TransportConfig) {
			if err := s.serve(ctx, cfg); err != nil {
				s.logger.Errorf(ctx, "transport server stopped with error: %s", err.Error())
			}
		}(server)
	}

	select {
	case <-s.sig:
		s.logger.Info(ctx, "quic server closed")
		return
	}
}

func (s *Server) serve(ctx context.Context, cfg *TransportConfig) (err error) {
	var internal internalServer

	if cfg.Name == "" {
		cfg.Name = fmt.Sprintf("unnamed-%s-server-%s", cfg.Type, uuid.NewString()[:8])
	}
	switch cfg.Type {
	case TypeQuic:
		internal = newQuicServer(s)
		s.internals.Store(cfg.Name, internal)
	case TypeTCP:
		internal = newTcpServer(s)
		s.internals.Store(cfg.Name, internal)
	default:
		err = fmt.Errorf("invalid server type: %s", cfg.Type)
		return
	}

	s.logger.Infof(ctx, "%s server %s started at %s:%d", cfg.Type, cfg.Name, cfg.Address, cfg.Port)

	if err = internal.Setup(ctx, cfg); err != nil {
		return err
	}

	for {
		select {
		case <-s.sig:
			s.logger.Info(ctx, fmt.Sprintf("%s server closed: %s", cfg.Type, cfg.Name))
			return
		default:
		}

		var (
			ss  *serverSession
			id  = uuid.NewString()
			ctx = context.WithValue(ctx, "id", id)
		)

		// accept connection
		ss, err = internal.Accept(ctx)
		if err != nil {
			s.logger.Errorf(ctx, "accept connection error: %s", err.Error())
			continue
		}
		ss.cfg = cfg
		ss.ref = internal
		ss.Session = &session.Session{
			Ctx:       ctx,
			Type:      session.Type(cfg.Type.String()),
			SessionId: fmt.Sprintf("%s-%v", strings.ToLower(ss.cfg.Type.String()), id),
		}

		go s.handleSession(ss)
	}
}

func (s *Server) handleSession(ss *serverSession) {
	var (
		err         error
		ctx, cancel = context.WithCancel(ss.Ctx)
	)

	defer func() {
		// close all goroutines
		cancel()
	}()

	// handshake
	err = s.handshake(ctx, ss)
	if err != nil {
		s.logger.Errorf(ctx, "handshake error: %s", err.Error())
		ss.CloseWithError(err)
		return
	}
	// store session
	s.sessions.Store(ss.IP, ss)

	routeAddress := fmt.Sprintf("%s/32", ss.IP)

	// set ctx key
	ctx = context.WithValue(ctx, consts.CtxKeyServerSession, ss)
	ctx = context.WithValue(ctx, consts.CtxKeyRouteAddress, routeAddress)
	ss.Ctx = ctx

	// register router
	ss.network.Router().Register(routeAddress, ss)

	var (
		ep error
	)

	// handle func call
	go s.handleFuncCallLoop(ss)

	defer func() {
		// release device
		err := ss.network.ReleaseDevice(ss.DispatchedDevice.CIDR)
		if err != nil {
			s.logger.Errorf(ss.Ctx, "release device error: %s", err.Error())
		}
		// unregister session
		s.sessions.Delete(ss.IP)
		// unregister router
		ss.network.Router().UnRegister(routeAddress)
		// close connection
		ss.CloseWithError(ep)
		s.logger.Infof(ss.Ctx, "%s session closed: %s", ss.IP, ss.SessionId)
	}()

	for {
		select {
		case <-s.sig:
			ep = errors.New("server closed")
			return
		case <-ctx.Done():
			ep = ctx.Err()
			return
		default:
			rwc, err := ss.ref.AcceptTransport(ss)
			if err != nil {
				s.logger.Errorf(ss.Ctx, "accept transport error: %s", err.Error())
				return
			}
			go s.handleTransport(ss, rwc)
		}
	}
}

func (s *Server) handleTransport(ss *serverSession, srcRwc io.ReadWriteCloser) {
	src := ss.IP
	dstSession, dst, err := s.negotiation(ss.Ctx, ss, srcRwc)
	if err != nil {
		_ = srcRwc.Close()
		s.logger.Errorf(ss.Ctx, "error during negotiation: %s", err.Error())
		return
	}
	dstRwc, err := ss.ref.GetDstTransportWriter(ss, dstSession)
	if err != nil {
		return
	}

	var (
		written int64
		name    = fmt.Sprintf("%s->%s", src, dst)
	)

	defer func() {
		s.logger.Infof(ss.Ctx, "proxy stopped: %s,written=%v", name, written)
	}()

	s.logger.Infof(ss.Ctx, "handle proxy start: %s", name)
	written, err = s.proxy(ss.Ctx, name, dstRwc, srcRwc)
	if err != nil {
		s.logger.Errorf(ss.Ctx, "proxy error: %s ,err=%v", name, err.Error())
	}
}

func (s *Server) handleFuncCallLoop(ss *serverSession) {
	var (
		err      error
		datagram []byte
	)

	defer func() {
		s.logger.Infof(ss.Ctx, "handle func call loop stopped: %s, reason=%v", ss.IP, err)
	}()

	for {
		select {
		case <-s.sig:
			err = errors.New("server closed")
			return
		case <-ss.Ctx.Done():
			err = ss.Ctx.Err()
			return
		default:
			datagram, err = ss.Receive(ss.Ctx)
			if err != nil {
				return
			}
			s.manager.PushEvent(ss, datagram)
		}
	}
}

func (s *Server) proxy(ctx context.Context, name string, dst io.WriteCloser, src io.ReadCloser) (written int64, err error) {
	var (
		buf = make([]byte, protocol.MaxTransportByteSize)
		nr  int
		er  error
	)

	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		case <-s.sig:
			s.logger.Infof(ctx, "proxy tunnel closed: %s", name)
			return
		default:
		}

		nr, er = src.Read(buf)
		if nr > 0 {
			// equals dst.Write(buf[0:nr]) when control not set
			nw, ew := dst.Write(buf[0:nr])
			// fmt.Printf("proxy %d->%d \n", nr, nw)
			if ew != nil {
				_ = dst.Close()
				return written, ew
			}
			if nw > 0 {
				written += int64(nw)
			}
		}
		if er != nil {
			if er != io.EOF {
				_ = src.Close()
				err = er
			}
			break
		}
	}
	return written, err
}

func (s *Server) handshake(ctx context.Context, ss *serverSession) (err error) {
	// Set a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func() {
		cancel()
	}()

	datagram, err := ss.Receive(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timeout while waiting for authentication data: traceid=%v", ctx.Value("id"))
			return
		}
		return
	}

	request, resp, err := s.auth.Auth(ctx, datagram)
	if err != nil {
		err = ErrUnauthorized.WithCause(err)
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

		_ = ss.Send(bs)
	}()

	// get network_id from request
	networkId, ok := request["network_id"].(string)
	if !ok {
		err = ErrInvalidParameter.WithCause(errors.New("network_id field from request not found"))
		return
	}

	ss.network, ok = network.GetManager().GetNetwork(networkId)
	if !ok {
		err = ErrResourceNotFound.WithCause(fmt.Errorf("network not found: id=%s", networkId))
		return
	}

	dev, err := ss.network.AcquireDevice(ctx, request)
	if err != nil {
		err = ErrResourceError.WithCause(fmt.Errorf("acquire device error: %s", err.Error()))
		return
	}
	s.logger.Infof(ctx, "dispatch device: id=%v cidr=%v", dev.Id, dev.CIDR)

	ip, _, _ := net.ParseCIDR(dev.CIDR)

	ss.IP = ip.To4().String()
	ss.NetworkId = networkId
	ss.NetworkInfo = map[string]any{
		"id":   networkId,
		"cidr": ss.network.CIDR,
		"mtu":  ss.network.MTU,
	}
	ss.DispatchedDevice = session.Device{
		Id:   dev.Id,
		Name: dev.Name,
		CIDR: dev.CIDR,
		MTU:  dev.MTU,
	}

	// build response
	resp["session"] = ss.Session

	// all these registration will be unregistered in acceptStreamLoop
	// when the connection is closed (can not accept new stream)
	s.logger.Infof(ctx, "handle connection, route=%s", ss.IP)
	return
}

func (s *Server) negotiation(_ context.Context, sess *serverSession, src io.ReadWriter) (dstSession *serverSession, dst string, err error) {
	// get dst ip by first packet, 10s timeout
	var (
		buf   = make([]byte, 15)
		ch    = make(chan []byte)
		first []byte
	)

	go func() {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		ch <- buf[:n]
	}()

	timer := time.NewTimer(time.Second * 3)
	select {
	case <-timer.C:
		err = fmt.Errorf("read first pkg timeout. src=%v", sess.IP)
		return
	case first = <-ch:
	}

	dst = string(first)

	v, ok := sess.network.Router().RouteString(dst)
	if !ok {
		err = fmt.Errorf("route not found: dst=%v", dst)
		return
	}

	// send ack (byte 1) to client
	_, _ = src.Write([]byte{1})
	dstSession, ok = v.(*serverSession)
	if !ok {
		err = fmt.Errorf("error session type: dst=%v", dst)
		return
	}
	return
}
