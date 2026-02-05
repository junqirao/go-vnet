package server

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"

	"go-vnet/common/protocol"
	"go-vnet/common/session"
	"go-vnet/vnet/server/consts"
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
		sig       chan struct{}
		manager   *Manager
		sessions  sync.Map // src : *Session
		// p2p
		p2pSignalingServer *p2pSignalingServer
		peerMappingVersion *atomic.Uint64
		// manager server
		ms ManagerServer
	}
	ManagerServer interface {
		AcquireDevice(ctx context.Context, ss *Session, subDeviceId uint64, payload map[string]any) (dev *session.Device, err error)
		PrivateKeyBySubDeviceId(ctx context.Context, id uint64, key string) (pri *rsa.PrivateKey, err error)
	}
	internalServer interface {
		io.Closer
		Setup(ctx context.Context, cfg *TransportConfig) (err error)
		Accept(ctx context.Context) (ss *Session, err error)
		AcceptTransport(session *Session) (rwc io.ReadWriteCloser, err error)
		GetDstTransportWriter(src *Session, dst *Session) (rwc io.ReadWriteCloser, err error)
	}
)

func NewServer(cfg *Config) *Server {
	s := &Server{
		cfg:                cfg,
		sig:                make(chan struct{}),
		manager:            NewManager(),
		peerMappingVersion: &atomic.Uint64{},
	}

	s.manager.RegisterHandler(
		funcPing,
		funcGetRouteData,
		funcGetP2PRelayInfo,
		funcGetP2PRelayMapping,
		funcRegisterP2PPeer,
	)

	// chainFunc := config.GetMappedConfig[[]auth.ServerAuthChainFunc](cfg,
	// 	ConfigKeyAuthChainFunc, []auth.ServerAuthChainFunc{})
	// s.auth = auth.NewServer(cfg.Auth, chainFunc)
	s.p2pSignalingServer = newP2PSignalingServer(cfg.P2P, s)
	return s
}

func (s *Server) Serve(ctx context.Context) (err error) {
	go s.checkStatusLoop()
	go s.backgroundUpdateMetricsLoop()
	defer func() {
		_ = s.manager.Close()
		_ = s.p2pSignalingServer.Close()
	}()

	// p2p signaling server
	if err = s.p2pSignalingServer.Run(ctx); err != nil {
		return
	}

	for _, server := range s.cfg.Servers {
		go func(cfg *TransportConfig) {
			if err := s.serve(ctx, cfg); err != nil {
				g.Log().Errorf(ctx, "transport server stopped with error: %s", err.Error())
			}
		}(server)
	}

	select {
	case <-s.sig:
		g.Log().Info(ctx, "quic server closed")
		return
	}
}

func (s *Server) RegisterManager(ms ManagerServer) {
	s.ms = ms
}

func (s *Server) checkStatusLoop() {
	for {
		select {
		case <-s.sig:
			return
		default:
		}
		s.sessions.Range(func(key, sess any) bool {
			ip := key.(string)
			ss := sess.(*Session)
			v, ok := ss.storage.Load(sessionStorageKeyLastPing)
			if !ok {
				// make sure the session could be removed if no ping packet received
				ss.storage.Store(ip, time.Now())
				return true
			}
			last := v.(time.Time)
			if time.Since(last) > time.Second*10 {
				g.Log().Infof(sess.(*Session).Ctx, "remove session %s, last ping time: %s", ip, last.String())
				sess.(*Session).Stop()
				// must delete the session
				s.sessions.Delete(ip)
			}
			return true
		})
		time.Sleep(time.Second * 10)
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

	g.Log().Infof(ctx, "%s server %s started at %s:%d", cfg.Type, cfg.Name, cfg.Address, cfg.Port)

	if err = internal.Setup(ctx, cfg); err != nil {
		return err
	}

	for {
		select {
		case <-s.sig:
			g.Log().Info(ctx, fmt.Sprintf("%s server closed: %s", cfg.Type, cfg.Name))
			return
		default:
		}

		var (
			ss *Session
			id = uuid.NewString()
		)

		ctx := context.WithValue(ctx, consts.CtxKeyId, id)
		ctx = context.WithValue(ctx, consts.CtxKeyServer, s)

		// accept connection
		ss, err = internal.Accept(ctx)
		if err != nil {
			g.Log().Errorf(ctx, "accept connection error: %s", err.Error())
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

func (s *Server) handleSession(ss *Session) {
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
		g.Log().Errorf(ctx, "handshake error: %s", err.Error())
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
			g.Log().Errorf(ss.Ctx, "release device error: %s", err.Error())
		}
		// unregister session
		s.sessions.Delete(ss.IP)
		// unregister router
		ss.network.Router().UnRegister(routeAddress)
		// close connection
		ss.CloseWithError(ep)
		// delete p2p peer mapping
		// add version make client re-sync
		s.peerMappingVersion.Add(1)
		g.Log().Infof(ss.Ctx, "%s session closed: %s", ss.IP, ss.SessionId)
	}()

	for {
		select {
		case <-s.sig:
			ep = errors.New("server closed")
			return
		case <-ss.sig:
			ep = errors.New("session closed")
			return
		case <-ctx.Done():
			ep = ctx.Err()
			return
		default:
			rwc, err := ss.ref.AcceptTransport(ss)
			if err != nil {
				g.Log().Errorf(ss.Ctx, "accept transport error: %s", err.Error())
				return
			}
			go s.handleTransport(ss, rwc)
		}
	}
}

func (s *Server) handleTransport(ss *Session, srcRwc io.ReadWriteCloser) {
	src := ss.IP
	dstSession, dst, err := s.negotiation(ss.Ctx, ss, srcRwc)
	if err != nil {
		_ = srcRwc.Close()
		g.Log().Errorf(ss.Ctx, "error during negotiation: %s", err.Error())
		return
	}
	dstRwc, err := dstSession.ref.GetDstTransportWriter(ss, dstSession)
	if err != nil {
		return
	}

	var (
		written int64
		name    = fmt.Sprintf("%s->%s", src, dst)
	)

	defer func() {
		g.Log().Infof(ss.Ctx, "proxy stopped: %s,written=%v", name, written)
	}()

	g.Log().Infof(ss.Ctx, "handle proxy start: %s", name)
	written, err = s.proxy(ss.Ctx, name, dstSession, ss, dstRwc, srcRwc)
	if err != nil {
		g.Log().Errorf(ss.Ctx, "proxy error: %s ,err=%v", name, err.Error())
	}
}

func (s *Server) handleFuncCallLoop(ss *Session) {
	var (
		err      error
		datagram []byte
	)

	defer func() {
		g.Log().Infof(ss.Ctx, "handle func call loop stopped: %s, reason=%v", ss.IP, err)
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
			_ = s.manager.HandleEvent(ss.Ctx, ss, datagram)
		}
	}
}

func (s *Server) proxy(ctx context.Context, name string, dstSess, srcSess *Session, dst io.WriteCloser, src io.ReadCloser) (written int64, err error) {
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
			g.Log().Infof(ctx, "proxy tunnel closed: %s", name)
			return
		default:
		}

		nr, er = src.Read(buf)
		srcSess.Metrics.TxBytes.Add(uint64(nr))
		srcSess.Metrics.TxPackets.Add(1)
		if nr > 0 {
			// equals dst.Write(buf[0:nr]) when control not set
			nw, ew := dst.Write(buf[0:nr])
			dstSess.Metrics.RxBytes.Add(uint64(nw))
			dstSess.Metrics.RxPackets.Add(1)
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

func (s *Server) handshake(ctx context.Context, ss *Session) (err error) {
	if s.ms == nil {
		err = ErrUnauthorized.WithCause(errors.New("manager server not registered"))
		return
	}

	// Set a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func() {
		cancel()
	}()

	datagram, err := ss.Receive(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timeout while waiting for authentication data: traceid=%v",
				ctx.Value(consts.CtxKeyId))
			return
		}
		return
	}

	header, payload, err := session.ParseRequest(datagram)
	if err != nil {
		return err
	}

	privateKey, err := s.ms.PrivateKeyBySubDeviceId(ctx, header.SubDeviceId, header.Key)
	if err != nil {
		return err
	}

	data, err := session.HandleRequest(ctx, header, privateKey, payload,
		func(ctx context.Context, req map[string]any) (resp map[string]any, err error) {
			resp = make(map[string]any)

			defer func() {
				if err != nil {
					resp["error"] = err.Error()
				}
			}()

			var dev *session.Device
			dev, err = s.ms.AcquireDevice(ctx, ss, header.SubDeviceId, req)
			if err != nil {
				err = ErrResourceError.WithCause(fmt.Errorf("acquire device error: %s", err.Error()))
				return
			}
			if ss.network == nil {
				err = ErrResourceError.WithCause(errors.New("internal error network not set"))
				return
			}

			networkId := ss.network.ID

			g.Log().Infof(ctx, "dispatch device: id=%v cidr=%v", dev.Id, dev.CIDR)

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
			g.Log().Infof(ctx, "handle connection, route=%s", ss.IP)
			return
		},
	)
	// always send data, when error caused, session
	// will be closed by Server.handleSession
	_ = ss.Send(data)
	return
}

func (s *Server) negotiation(_ context.Context, sess *Session, src io.ReadWriter) (dstSession *Session, dst string, err error) {
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
	dstSession, ok = v.(*Session)
	if !ok {
		err = fmt.Errorf("error session type: dst=%v", dst)
		return
	}
	return
}
