package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/vnet/server/consts"
	"go-vnet/vnet/server/network"
	"go-vnet/vnet/session"
)

var (
	ErrUnauthorized     = session.NewError("unauthorized", 401)
	ErrInvalidParameter = session.NewError("invalid parameter", 400)
	ErrResourceNotFound = session.NewError("resource not found", 404)
	ErrResourceError    = session.NewError("resource error", 406)
)

type (
	Server struct {
		internal internalServer
		cfg      *Config
		logger   logger.Logger
		sig      chan struct{}
		auth     *auth.Server
		manager  *Manager
		sessions sync.Map // src : *session.ServerSession
	}
	internalServer interface {
		io.Closer
		Run(ctx context.Context) (err error)
		Accept(ctx context.Context) (sr session.SendReceiveCloser, conn any, err error)
		HandleProxy(ctx context.Context, session *serverSession) (err error)
	}
	serverSession struct {
		*session.Session
		session.SendReceiveCloser
		network *network.Network
		conn    any
	}
)

func NewServer(cfg *Config) *Server {
	if cfg.Name == "" {
		cfg.Name = fmt.Sprintf("unnamed-%s-server-%s", cfg.Type, uuid.NewString()[:8])
	}

	s := &Server{
		cfg:     cfg,
		logger:  config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		sig:     make(chan struct{}),
		manager: NewManager(),
	}

	chainFunc := config.GetMappedConfig[[]auth.ServerAuthChainFunc](cfg,
		ConfigKeyAuthChainFunc, []auth.ServerAuthChainFunc{})
	s.auth = auth.NewServer(cfg.Auth, chainFunc)

	switch cfg.Type {
	case TypeQuic:
		s.internal = newQuicServer(s)
	default:
		panic(fmt.Sprintf("invalid server type: %s", cfg.Type))
	}
	return s
}

func (s *Server) Serve(ctx context.Context) (err error) {
	if err = s.internal.Run(ctx); err != nil {
		return
	}
	defer func() {
		_ = s.manager.Close()
		_ = s.internal.Close()
	}()
	s.logger.Infof(ctx, "%s server %s started at %s:%d", s.cfg.Type, s.cfg.Name, s.cfg.Address, s.cfg.Port)

	go func() {
		_ = s.manager.ProcessFuncCallLoop(ctx)
	}()

	for {
		select {
		case <-s.sig:
			s.logger.Info(ctx, "quic server closed")
			return
		default:
		}

		var (
			sr   session.SendReceiveCloser
			conn any
		)

		// accept connection
		sr, conn, err = s.internal.Accept(ctx)
		if err != nil {
			s.logger.Errorf(ctx, "accept connection error: %s", err.Error())
			return
		}

		ss := &serverSession{
			conn:              conn,
			SendReceiveCloser: sr,
		}
		// handshake
		ss.Session, ss.network, err = s.handshake(ctx, sr)
		if err != nil {
			s.logger.Errorf(ctx, "handshake error: %s", err.Error())
			sr.CloseWithError(err)
			continue
		}

		routeAddress := fmt.Sprintf("%s/32", ss.IP)

		// set ctx key
		ctx = context.WithValue(ctx, consts.CtxKeyServerSession, ss)
		ctx = context.WithValue(ctx, consts.CtxKeyRouteAddress, routeAddress)

		// register router
		ss.network.Router().Register(routeAddress, ss)

		// func call loop
		go s.handleFuncCallLoop(ctx, ss)

		//
		go func() {
			var (
				ep error
			)

			defer func() {
				// release device
				err := ss.network.ReleaseDevice(ss.DispatchedDevice.CIDR)
				if err != nil {
					s.logger.Errorf(ctx, "release device error: %s", err.Error())
				}
				// unregister session
				s.sessions.Delete(ss.IP)
				// unregister router
				ss.network.Router().UnRegister(routeAddress)
				// close connection
				sr.CloseWithError(ep)
			}()

			ep = s.internal.HandleProxy(ctx, ss)
		}()
	}
}

func (s *Server) proxy(ctx context.Context, name string, dst io.Writer, src io.Reader) (written int64, err error) {
	var (
		buf = make([]byte, 65535)
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
			fmt.Printf("proxy %d->%d \n", nr, nw)
			if ew != nil {
				return written, ew
			}
			if nw > 0 {
				written += int64(nw)
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

func (s *Server) handshake(ctx context.Context, sr session.SendReceiveCloser) (sess *session.Session, nwk *network.Network, err error) {
	// Set a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func() {
		cancel()
	}()

	datagram, err := sr.Receive(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timeout while waiting for authentication data: %s", sess.SessionId)
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

		_ = sr.Send(bs)
	}()

	// get network_id from request
	networkId, ok := request["network_id"].(string)
	if !ok {
		err = ErrInvalidParameter.WithCause(errors.New("network_id field from request not found"))
		return
	}

	nwk, ok = network.GetManager().GetNetwork(networkId)
	if !ok {
		err = ErrResourceNotFound.WithCause(fmt.Errorf("network not found: id=%s", networkId))
		return
	}

	dev, err := nwk.AcquireDevice(ctx, request)
	if err != nil {
		err = ErrResourceError.WithCause(fmt.Errorf("acquire device error: %s", err.Error()))
		return
	}
	s.logger.Infof(ctx, "dispatch device: id=%v cidr=%v", dev.Id, dev.CIDR)

	ip, _, _ := net.ParseCIDR(dev.CIDR)

	sess = &session.Session{
		IP:        ip.To4().String(),
		Ctx:       ctx,
		Type:      session.Type(s.cfg.Type),
		SessionId: uuid.New().String(),
		NetworkId: networkId,
		NetworkInfo: map[string]any{
			"id":   networkId,
			"cidr": nwk.CIDR,
			"mtu":  nwk.MTU,
		},
		DispatchedDevice: session.Device{
			Id:   dev.Id,
			Name: dev.Name,
			CIDR: dev.CIDR,
			MTU:  dev.MTU,
		},
	}

	// build response
	resp["session"] = sess

	// register session
	s.sessions.Store(sess.IP, sess)
	// all these registration will be unregistered in acceptStreamLoop
	// when the connection is closed (can not accept new stream)
	s.logger.Infof(ctx, "handle connection, route=%s", sess.IP)
	return
}

func (s *Server) handleFuncCallLoop(ctx context.Context, ss *serverSession) {
	var (
		err      error
		datagram []byte
	)

	defer func() {
		s.logger.Infof(ctx, "handle func call loop stopped: %s, reason=%v", ss.IP, err)
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
			datagram, err = ss.Receive(ctx)
			if err != nil {
				return
			}
			s.manager.PushEvent(ss, datagram)
		}
	}
}
