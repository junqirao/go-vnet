package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/server/connection"
	"go-vnet/server/network"
)

const (
	ManagerMark = 0x01
)

type (
	Server interface {
		Serve(ctx context.Context) error
		Close() (err error)
	}
	transportServer struct {
		ctx            context.Context
		sig            chan struct{}
		logger         logger.Logger
		auth           *auth.Server
		mgrWriter      sync.Map
		funcCallEvents chan *FuncCallEvent
		mgrWg          sync.WaitGroup
		*ServerConfig
		Server
	}
	FuncCallEvent struct {
		Info    *connection.Info
		Payload []byte
	}
)

func NewTransportServer(funcCallEventChan chan *FuncCallEvent, cfg *ServerConfig) Server {
	s := &transportServer{
		logger:         config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger),
		ServerConfig:   cfg,
		sig:            make(chan struct{}),
		mgrWriter:      sync.Map{},
		funcCallEvents: funcCallEventChan,
	}

	s.auth = auth.NewServer(cfg.Auth, []auth.ServerAuthChainFunc{s.authChainFunc})

	switch cfg.Type {
	case TypeQuic:
		s.Server = newQuicServer(s)
	}
	return s
}

func (s *transportServer) Serve(ctx context.Context) error {
	if s.Server == nil {
		return fmt.Errorf("transport server type not supported: %s", s.Type)
	}
	s.ctx = ctx
	return s.Server.Serve(ctx)
}

func (s *transportServer) Close() error {
	close(s.sig)
	s.mgrWriter.Range(func(key, value any) bool {
		info := value.(*connection.Info)
		_ = info.Close()
		return true
	})
	s.mgrWg.Wait()
	return nil
}

func (s *transportServer) handleConn(ctx context.Context, conn io.ReadWriteCloser, info *connection.Info) {
	if conn == nil {
		return
	}

	// read stream max 10s for first pkg
	var (
		buf       = make([]byte, 15)
		ch        = make(chan []byte)
		dst       io.Writer
		first     []byte
		writeBack byte = 0
		isManager bool
		cancel    context.CancelFunc
	)

	ctx, cancel = context.WithCancel(ctx)

	defer func() {
		if isManager {
			return
		}
		if conn != nil {
			_ = conn.Close()
		}
		cancel()
	}()

	go func() {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		ch <- buf[:n]
	}()

	timer := time.NewTimer(time.Second * 10)
	select {
	case <-timer.C:
		s.logger.Errorf(ctx, "read first pkg timeout. src=%v", info.Src)
		return
	case first = <-ch:
	}

	var (
		from = string(first)
		to   string
	)

	if len(first) == 1 && first[0] == ManagerMark {
		dst = conn
		writeBack = 1
		isManager = true
	} else {
		res, ok := info.Network.Router().RouteString(from)
		if ok {
			if ci, ok := res.(*connection.Info); ok {
				to = ci.Src
				dst = ci.GetRWC()
				writeBack = 1
			}
		}
	}

	_, err := conn.Write([]byte{writeBack})
	if err != nil {
		s.logger.Errorf(ctx, "write back failed. src=%v, dst=%v, writeBack=%v, err=%v", info.Src, to, writeBack, err)
		return
	}

	// register manager connection
	if isManager {
		s.mgrWriter.Store(info.Device.CIDR, info)
		s.startManagerReader(info)
		s.logger.Infof(ctx, "handle manager connection: src=%v", info.Src)
		return
	}

	s.logger.Infof(ctx, "handle flow start: %s -> %s", info.Src, to)

	if dst == nil {
		s.logger.Errorf(ctx, "no connection to dst: %s", to)
		return
	}

	// block and redirect flow to s.dst
	if _, err = s.handleFlowProxy(ctx, fmt.Sprintf("%s -> %s", info.Src, to), dst, conn, make([]byte, s.MTU)); err != nil {
		s.logger.Errorf(ctx, "handle flow stopped. src=%v error: %s", info.Src, err.Error())
		return
	}
}

func (s *transportServer) handleFlowProxy(ctx context.Context, name string, dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		case <-s.sig:
			s.logger.Infof(s.ctx, "proxy tunnel closed: %s", name)
			return
		default:
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			fmt.Printf("nw=%v,ew=%v,data=%v\n", nw, ew, buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
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

func (s *transportServer) authAndRegisterRouter(ctx context.Context, conn any,
	receive func(ctx context.Context) ([]byte, error),
	send func(data []byte) error) (ci *connection.Info, err error) {
	// Set a context with a 10-second timeout
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer func() {
		cancel()
	}()

	type remote interface {
		RemoteAddr() net.Addr
	}
	var rem = ""
	if r, ok := conn.(remote); ok {
		rem = r.RemoteAddr().String()
	}

	datagram, err := receive(ctx)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			err = fmt.Errorf("timeout while waiting for authentication data: %s", rem)
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

		_ = send(bs)
	}()

	// 通过network_id获取network
	networkID, ok := request["network_id"].(string)
	if !ok {
		err = fmt.Errorf("network_id field from request not found")
		s.logger.Error(ctx, err.Error())
		return
	}

	nwk, ok := network.GetManager().GetNetwork(networkID)
	if !ok {
		err = fmt.Errorf("network not found: id=%s", networkID)
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
	src := fmt.Sprintf("%s/32", ip.To4().String())

	ci = &connection.Info{
		Network: nwk,
		Device:  dev,
		Src:     src,
	}

	if err = nwk.Router().Register(ctx, src, ci); err != nil {
		s.logger.Errorf(ctx, "register router error: %s", err.Error())
		return
	}

	s.logger.Infof(ctx, "handle connection: remote_addr=%s,route=%s", rem, src)
	return
}

func (s *transportServer) authChainFunc(ctx context.Context, request map[string]any, resp map[string]any) (err error) {
	s.logger.Infof(ctx, "auth chain func: %v", request)
	return
}

func (s *transportServer) startManagerReader(info *connection.Info) {
	s.mgrWg.Add(1)
	go func() {
		defer s.mgrWg.Done()
		defer s.mgrWriter.Delete(info.Device.CIDR)
		defer func() {
			_ = info.GetRWC().Close()
		}()

		buf := make([]byte, 4096)
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-s.sig:
				return
			default:
			}

			n, err := info.GetRWC().Read(buf)
			if err != nil {
				if err != io.EOF {
					s.logger.Errorf(s.ctx, "manager read error: %s, src=%s", err.Error(), info.Src)
				}
				return
			}

			if n > 0 {
				payload := make([]byte, n)
				copy(payload, buf[:n])
				select {
				case s.funcCallEvents <- &FuncCallEvent{Info: info, Payload: payload}:
				case <-s.ctx.Done():
					return
				case <-time.After(time.Second):
					s.logger.Errorf(s.ctx, "funcCallEvents channel full, dropping packet from %s", info.Src)
				}
			}
		}
	}()
}
