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
	"go-vnet/server/network"
)

var (
	ErrUnauthorized     = NewError("unauthorized", 401)
	ErrInvalidParameter = NewError("invalid parameter", 400)
	ErrResourceNotFound = NewError("resource not found", 404)
	ErrResourceError    = NewError("resource error", 406)
)

type (
	Server struct {
		internal internalServer
		cfg      *Config
		logger   logger.Logger
		sig      chan struct{}
		auth     *auth.Server
		manager  *Manager
		sessions sync.Map // src : *Session
	}
	internalServer interface {
		io.Closer
		Run(ctx context.Context) (err error)
		Accept(ctx context.Context) (sr SendReceiveCloser, conn any, err error)
		HandleProxy(ctx context.Context, session *Session) (err error)
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
			sr      SendReceiveCloser
			conn    any
			session *Session
		)

		// accept connection
		sr, conn, err = s.internal.Accept(ctx)
		if err != nil {
			s.logger.Errorf(ctx, "accept connection error: %s", err.Error())
			return
		}

		// handshake
		session, err = s.handshake(ctx, sr, conn)
		if err != nil {
			s.logger.Errorf(ctx, "handshake error: %s", err.Error())
			session.CloseWithError(err)
			return
		}

		routeAddress := fmt.Sprintf("%s/32", session.IP)

		// register router
		if err = session.Network.Router().Register(ctx, routeAddress, session); err != nil {
			session.CloseWithError(err)
			return
		}

		// func call loop
		go s.handleFuncCallLoop(ctx, session)

		//
		go func() {
			var (
				ep error
			)

			defer func() {
				// release device
				err := session.Network.ReleaseDevice(session.DispatchedDevice)
				if err != nil {
					s.logger.Errorf(ctx, "release device error: %s", err.Error())
				}
				// unregister session
				s.sessions.Delete(session.IP)
				// unregister router
				if err := session.Network.Router().Delete(ctx, routeAddress); err != nil {
					s.logger.Errorf(ctx, "unregister router error: %s", err.Error())
				}
				// close connection
				session.CloseWithError(ep)
			}()

			ep = s.internal.HandleProxy(ctx, session)
		}()
	}
}

func (s *Server) proxy(ctx context.Context, name string, dst io.Writer, src io.Reader) (written int64, err error) {
	buf := make([]byte, s.cfg.MTU*100)

	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		case <-s.sig:
			s.logger.Infof(ctx, "proxy tunnel closed: %s", name)
			return
		default:
		}

		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
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

func (s *Server) handshake(ctx context.Context, sr SendReceiveCloser, conn any) (session *Session, err error) {
	// Set a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func() {
		cancel()
	}()

	var (
		remote string
	)

	type remotable interface {
		RemoteAddr() net.Addr
	}
	if rem, ok := conn.(remotable); ok {
		remote = rem.RemoteAddr().String()
	}

	datagram, err := sr.Receive(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timeout while waiting for authentication data: %s", remote)
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

	nwk, ok := network.GetManager().GetNetwork(networkId)
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

	resp["device"] = dev
	// register router
	ip, _, _ := net.ParseCIDR(dev.CIDR)

	session = &Session{
		SendReceiveCloser: sr,
		Type:              s.cfg.Type,
		Id:                uuid.NewString(),
		Network:           nwk,
		DispatchedDevice:  dev,
		IP:                ip.To4().String(),
		Conn:              conn,
	}

	// register session
	s.sessions.Store(session.IP, session)
	// all these registration will be unregistered in acceptStreamLoop
	// when the connection is closed (can not accept new stream)
	s.logger.Infof(ctx, "handle connection: remote_addr=%s,route=%s", remote, session.IP)
	return
}

func (s *Server) handleFuncCallLoop(ctx context.Context, session *Session) {
	var (
		err      error
		datagram []byte
	)

	defer func() {
		s.logger.Infof(ctx, "handle func call loop stopped: %s, reason=%s", session.IP, err.Error())
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
			datagram, err = session.Receive(ctx)
			if err != nil {
				return
			}
			s.manager.PushEvent(session, datagram)
		}
	}
}
