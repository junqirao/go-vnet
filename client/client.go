package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/songgao/water/waterutil"

	"go-vnet/client/transport"
	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/connection"
	"go-vnet/common/device"
	"go-vnet/common/logger"
	"go-vnet/common/router"
)

const (
	maxRetryCount   = 5
	configKeyLogger = "logger"
)

type Client struct {
	ctx     context.Context
	dev     device.IDevice
	bufPool sync.Pool
	sig     chan struct{}
	logger  logger.Logger

	// router
	router router.Router

	// connection manager
	cm *connection.Manager

	// auth
	auth *auth.Client

	// config
	cfg *Config

	// joined
	joined transport.JoinNetworkResponse
}

func NewClient(cfg Config) (c *Client, err error) {
	c = &Client{
		sig:    make(chan struct{}),
		cfg:    &cfg,
		logger: config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger),
		router: router.NewRouter(),
	}

	c.auth = auth.NewClient(cfg.Auth)
	return
}

func (c *Client) SetLogger(l logger.Logger) {
	c.logger = l
}

func (c *Client) SetRouter(r router.Router) {
	c.router = r
}

func (c *Client) Run(ctx context.Context) (err error) {
	c.ctx = ctx
	// 1. connect to server
	c.logger.Infof(ctx, "connect to server: %s", c.cfg.Server)
	t, err := transport.NewTransport(ctx,
		transport.NewConfig(
			transport.WithInsecureSkipVerify(c.cfg.InsecureSkipVerify),
			transport.WithAddress(c.cfg.Server),
			transport.WithAuthenticationPayload(map[string]any{
				"network_id": c.cfg.NetworkId,
			}),
		),
		c.auth,
	)
	if err != nil {
		err = fmt.Errorf("failed to connect to server: %w", err)
		return
	}

	// 2. setup device
	if c.dev != nil {
		_ = c.dev.Close()
	}
	c.joined = t.JoinedNetwork()
	c.logger.Infof(ctx, "joined network: %+v", c.joined)
	if c.cfg.DeviceType != "" {
		c.joined.Device.Type = device.Type(c.cfg.DeviceType)
	}
	c.dev = device.NewTunDevice(c.joined.Device)
	if err = c.dev.Setup(); err != nil {
		return
	}

	c.bufPool.New = func() any {
		return make([]byte, c.joined.Device.MTU)
	}

	c.cm = connection.NewManager(ctx, func(dst string) (io.ReadWriteCloser, error) {
		return t.Connect(dst)
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
			err = fmt.Errorf("failed to read device: %w", err)
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
	if n == 0 {
		return
	}

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
		if err = c.router.Register(c.ctx, fmt.Sprintf("%s/32", dst), rwc); err != nil {
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
