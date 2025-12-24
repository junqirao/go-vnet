package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/device"
	"go-vnet/server/network"
)

type (
	Server interface {
		Serve(ctx context.Context) error
		Close() (err error)
	}
	transportServer struct {
		ctx    context.Context
		sig    chan struct{}
		logger logger.Logger
		auth   *auth.Server
		*ServerConfig
		Server
	}
	connectionInfo struct {
		network *network.Network
		device  *device.Device
		src     string
	}
)

func NewTransportServer(cfg *ServerConfig) Server {
	s := &transportServer{
		logger:       config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger),
		ServerConfig: cfg,
		sig:          make(chan struct{}),
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

func (s *transportServer) handleFlowProxy(name string, dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	for {
		select {
		case <-s.sig:
			s.logger.Infof(s.ctx, "proxy tunnel closed: %s", name)
			return
		default:
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
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
	send func(data []byte) error) (ci connectionInfo, err error) {
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
		err = fmt.Errorf("network not found: network_id=%s", networkID)
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
	src := dev.CIDR
	if err = nwk.Router().Register(src, conn); err != nil {
		s.logger.Errorf(ctx, "register router error: %s", err.Error())
		return
	}

	ci = connectionInfo{
		network: nwk,
		device:  dev,
		src:     src,
	}

	s.logger.Infof(ctx, "handle connection: remote_addr=%s,route=%s", rem, src)
	return
}

func (s *transportServer) authChainFunc(ctx context.Context, request map[string]any, resp map[string]any) (err error) {
	s.logger.Infof(ctx, "auth chain func: %v", request)
	return
}
