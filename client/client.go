package client

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/songgao/water/waterutil"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/device"
	"go-vnet/common/logger"
	"go-vnet/common/router"
	"go-vnet/server"
)

type (
	internalClient interface {
		// Dial to server and create manager with established connection
		Dial(ctx context.Context) (session *Session, err error)
		// SendToServer send flow to server by dst ip address
		SendToServer(dst string, buf []byte, n int) (err error)
	}
	Client struct {
		internal internalClient
		ctx      context.Context
		sig      chan struct{}
		cfg      *Config
		logger   logger.Logger
		auth     *auth.Client
		manager  *Manager
		router   router.Router
		bufPool  sync.Pool
		dev      device.IDevice
		src      string
	}
)

func NewClient(cfg *Config) *Client {
	c := &Client{
		cfg:    cfg,
		sig:    make(chan struct{}),
		logger: config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		auth:   auth.NewClient(cfg.Auth),
		router: router.NewRouter(),
	}
	switch cfg.Type {
	case TypeQuic:
		c.internal = NewQuicClient(c)
	default:
		panic(fmt.Sprintf("invalid client type: %s", cfg.Type))
	}
	return c
}

func (c *Client) Run(ctx context.Context) (err error) {
	c.ctx = ctx

	// 1. connect to server
	c.logger.Infof(ctx, "connect to server %s:%d", c.cfg.Address, c.cfg.Port)
	session, err := c.Dial(ctx)
	if err != nil {
		return
	}

	// 2. setup device
	if c.dev != nil {
		_ = c.dev.Close()
	}
	if c.cfg.DeviceType != "" {
		session.DispatchedDevice.Type = device.Type(c.cfg.DeviceType)
	}
	c.dev = device.NewTunDevice(session.DispatchedDevice)
	if err = c.dev.Setup(); err != nil {
		err = fmt.Errorf("failed to setup device: %w", err)
		_ = session.Close()
		return
	}
	ip, _, _ := net.ParseCIDR(session.DispatchedDevice.CIDR)
	c.src = ip.To4().String()
	c.logger.Infof(ctx, "dispatched device ip=%s,mtu=%d", c.src,
		session.DispatchedDevice.MTU)

	// 3. sync router loop
	go c.syncRouterLoop()

	// 4. handle rx flow

	// 5. block and handle tx flow
	return c.HandleTX()
}

func (c *Client) Dial(ctx context.Context) (session *Session, err error) {
	return c.internal.Dial(ctx)
}

func (c *Client) syncRouter(ctx context.Context) (err error) {
	// ping
	resp, err := c.manager.CallFunc(ctx, server.FuncNamePing)
	if err != nil {
		c.logger.Errorf(c.ctx, "failed to execute ping to server: %s", err.Error())
		return
	}
	// c.logger.Infof(ctx, "ping latency: %.2fms", resp.Cost)

	// update router if hash changed
	current := c.router.Hash()
	if resp.Data == current {
		return
	}

	// get router data from server
	c.logger.Infof(c.ctx, "router hash changed, current: %s, server: %s", current, resp.Data)
	resp, err = c.manager.CallFunc(ctx, server.FuncNameGetRouterData)
	if err != nil {
		c.logger.Errorf(c.ctx, "failed to execute get router data from server: %s", err.Error())
		return
	}

	// decode and restore
	if data, ok := resp.Data.(string); ok && len(data) > 0 {
		var bs []byte
		bs, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			c.logger.Errorf(c.ctx, "failed to decode router data from server: %s", err.Error())
			return
		}
		if err = c.router.Restore(c.ctx, bs); err != nil {
			c.logger.Errorf(c.ctx, "failed to restore router from server: %s", err.Error())
			return
		}

		c.logger.Infof(c.ctx, "router synced from server, data: %d bytes, length: %d",
			len(data), c.router.Len())
	}
	return
}

func (c *Client) syncRouterLoop() {
	c.logger.Infof(c.ctx, "sync router loop started.")
	for {
		select {
		case <-c.sig:
			c.logger.Infof(c.ctx, "manager connection closed.")
			return
		case <-c.ctx.Done():
			c.logger.Infof(c.ctx, "manager connection closed.")
			return
		default:
		}

		// sync router
		if err := c.syncRouter(context.Background()); err != nil {
			c.logger.Errorf(c.ctx, "failed to sync router: %s", err.Error())
		}

		// sleep interval
		time.Sleep(time.Second * 5)
	}
}

func (c *Client) HandleTX() error {
	cfg := c.dev.GetConfig()
	c.bufPool = sync.Pool{New: func() any {
		return make([]byte, cfg.MTU)
	}}

	for {
		select {
		case <-c.sig:
			return nil
		default:
		}
		buf := c.bufPool.Get().([]byte)
		n, err := c.dev.Read(buf)
		if err != nil {
			err = fmt.Errorf("failed to read device: %w", err)
			return err
		}
		if n == 0 {
			continue
		}

		dst := waterutil.IPv4Destination(buf[:n]).String()

		// drop current loopback packet
		if dst == "127.0.0.1" || dst == c.src {
			continue
		}

		if err = c.SendToServer(dst, buf, n); err != nil {
			return err
		}
	}
}

func (c *Client) SendToServer(dst string, buf []byte, n int) (err error) {
	defer func() {
		c.bufPool.Put(buf)
	}()
	_, ok := c.router.Route(dst)
	if !ok {
		return
	}
	return c.internal.SendToServer(dst, buf, n)
}
