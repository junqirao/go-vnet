package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sync"

	"github.com/songgao/water/waterutil"

	"go-vnet/common/auth"
	"go-vnet/common/logger"
	"go-vnet/common/router"
	"go-vnet/config"
	"go-vnet/device"
)

const (
	funcNameJoinNetwork = "join_network"
	maxRetryCount       = 5
)

type Client struct {
	ctx       context.Context
	server    string
	networkId string
	dev       device.Device
	auth      *auth.Handler
	bufPool   sync.Pool
	sig       chan struct{}
	logger    logger.Logger

	// router
	router router.Router

	// connection manager
	cm *ConnectionManager
}

type AuthConfig struct {
	config.Auth
	// method http only
	Url         string            `json:"url,omitempty"`
	HTTPHeaders map[string]string `json:"http_headers,omitempty"`
}

type Config struct {
	NetworkId string     `json:"network_id"`
	Server    string     `json:"server"`
	Auth      AuthConfig `json:"auth"`
}

type JoinNetworkResponse struct {
	Device device.Config `json:"device"`
	Server string        `json:"server"`
}

func NewClient(cfg Config, networkId string) (c *Client, err error) {
	c = &Client{
		networkId: networkId,
		sig:       make(chan struct{}),
	}
	// auth
	var au auth.AuthorizedHandler
	switch cfg.Auth.Type {
	case config.AuthTypeSimplePassword:
		au = auth.NewSimplePasswordAuthenticator(cfg.Auth.Password, cfg.Auth.Md5Salt)
	case config.AuthTypeRSA:
		// todo
		// au, err = auth.NewRSAAuthenticator()
		// if err != nil {
		// 	return
		// }
		fallthrough
	default:
		err = fmt.Errorf("auth type not supported: %s", cfg.Auth.Type)
		return
	}

	var method auth.HandleFunc
	switch cfg.Auth.Method {
	case config.AuthMethodHTTP:
		method = auth.NewHTTPHandler(cfg.Auth.Url, cfg.Auth.HTTPHeaders)
	case config.AuthMethodIO:
		// todo
	default:
		err = fmt.Errorf("auth method not supported: %s", cfg.Auth.Method)
		return
	}
	c.auth = auth.NewHandler(au, method)
	return
}

func NewClientWithAuthorizedHandler(networkId string, au *auth.Handler) (c *Client, err error) {
	c = &Client{
		networkId: networkId,
		sig:       make(chan struct{}),
		auth:      au,
		logger:    logger.DefaultLogger,
	}
	return
}

func (c *Client) SetLogger(l logger.Logger) {
	c.logger = l
}

func (c *Client) SetRouter(r router.Router) {
	c.router = r
}

func (c *Client) joinNetwork(ctx context.Context, id string) (resp JoinNetworkResponse, err error) {
	err = c.auth.CallPtr(ctx, funcNameJoinNetwork, map[string]any{
		"network_id": id,
	}, &resp)
	return
}

func (c *Client) Run(ctx context.Context) (err error) {
	c.ctx = ctx
	// 1. join network
	join, err := c.joinNetwork(ctx, c.networkId)
	if err != nil {
		return
	}
	c.server = join.Server

	c.logger.Infof(ctx, "received connect info: %+v", join)

	// 2. setup device
	if c.dev != nil {
		_ = c.dev.Close()
	}
	c.dev = device.NewTunDevice(join.Device)
	if err = c.dev.Setup(); err != nil {
		return
	}

	c.bufPool.New = func() any {
		return make([]byte, join.Device.MTU)
	}

	// 3. connect to server
	server, err := netip.ParseAddrPort(c.server)
	if err != nil {
		c.logger.Errorf(ctx, "parse server address error: err=%s, server=%s", err.Error(), c.server)
		return
	}

	var (
		address = server.Addr().String()
		port    = server.Port()
	)

	c.logger.Infof(ctx, "connect to server: %s:%d", address, port)
	transport, err := NewTransport(ctx, TransportTypeQuic, address, int(port), c.auth, &join.Device)
	if err != nil {
		return
	}
	c.cm = NewConnectionManager(ctx, transport.Connect)
	c.cm.SetLogger(c.logger)

	// 4. block and read device
txLoop:
	for {
		select {
		case <-c.sig:
			break txLoop
		default:
		}
		buf := c.bufPool.Get().([]byte)
		n, err := c.dev.Read(buf)
		if err != nil {
			return err
		}
		c.handleTX(buf, n)
	}

	return errors.New("client close")
}

func (c *Client) handleTX(buf []byte, n int) {
	defer func() {
		c.bufPool.Put(buf)
	}()

	dst := waterutil.IPv4Destination(buf[:n]).String()
	v, ok := c.router.RouteString(dst)
	if !ok || v == nil {
		// drop
		return
	}

	var rwc io.ReadWriteCloser

	createFunc := func() (err error) {
		rwc, err = c.cm.Get(dst)
		if err != nil {
			c.logger.Errorf(c.ctx, "failed to get connection %s: %s", dst, err.Error())
			return
		}
		if err = c.router.Register(fmt.Sprintf("%s/32", dst), rwc); err != nil {
			c.logger.Errorf(c.ctx, "failed to register router: %v", err)
			return
		}
		return
	}

	rwc, ok = v.(io.ReadWriteCloser)
	if !ok {
		_ = createFunc()
	}

	retryCount := 0
	for retryCount < maxRetryCount {
		var err error
		if rwc != nil {
			_, err = rwc.Write(buf[:n])
			if err == nil {
				return
			}
			c.cm.Del(dst)
		}

		c.logger.Errorf(c.ctx, "failed to write to %s: %v", dst, err)
		// 尝试创建新的 stream
		c.logger.Infof(c.ctx, "try create stream...")
		if err = createFunc(); err != nil {
			break
		}
		retryCount++
	}
}

func (c *Client) Close() error {
	close(c.sig)
	return nil
}
