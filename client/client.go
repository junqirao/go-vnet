package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sync"

	"github.com/songgao/water/waterutil"

	"go-vnet/client/transport"
	"go-vnet/common/logger"
	"go-vnet/common/router"
	"go-vnet/config"
	"go-vnet/device"
	"go-vnet/server/auth"
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
	bufPool   sync.Pool
	sig       chan struct{}
	logger    logger.Logger

	// router
	router router.Router

	// connection manager
	cm *ConnectionManager

	// auth
	auth *auth.Client
}

type AuthConfig struct {
	config.Auth
	// method http only
	Url         string            `json:"url,omitempty"`
	HTTPHeaders map[string]string `json:"http_headers,omitempty"`
}

type Config struct {
	NetworkId string      `json:"network_id"`
	Server    string      `json:"server"`
	Auth      auth.Config `json:"auth"`
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

	var encoder auth.Encoder
	switch cfg.Auth.Type {
	case auth.TypeRSA:
		var opts []auth.RSAEncoderOption
		if cfg.Auth.PublicKey != "" {
			opts = append(opts, auth.WithPublicKey(cfg.Auth.PublicKey))
		}
		if cfg.Auth.PrivateKey != "" {
			opts = append(opts, auth.WithPrivateKey(cfg.Auth.PrivateKey))
		}
		encoder = auth.NewRsaEncoder(opts...)
	default:
		c.logger.Infof(c.ctx, "use default auth type: %s", auth.TypeSimplePassword)
		encoder = auth.NewSimplePasswordEncoder(cfg.Auth.Password)
	}
	c.auth = auth.NewClient(encoder)
	return
}

func (c *Client) SetLogger(l logger.Logger) {
	c.logger = l
}

func (c *Client) SetRouter(r router.Router) {
	c.router = r
}

func (c *Client) joinNetwork(ctx context.Context, id string) (resp JoinNetworkResponse, err error) {
	// err = c.auth.CallPtr(ctx, funcNameJoinNetwork, map[string]any{
	// 	"network_id": id,
	// }, &resp)
	m, err := c.auth.Auth(ctx,
		map[string]any{
			"network_id": id,
		},
		func(ctx context.Context, in []byte) (out []byte, err error) {
			return
		},
	)
	if err != nil {
		return
	}
	resp = JoinNetworkResponse{}
	bs, _ := json.Marshal(m)
	_ = json.Unmarshal(bs, &resp)
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
	t, err := transport.NewTransport(ctx, transport.TypeQuic, address, int(port), c.auth)
	if err != nil {
		return
	}
	c.cm = NewConnectionManager(ctx, func(dst string) (io.ReadWriteCloser, error) {
		return t.Connect(&join.Device, dst)
	})
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
