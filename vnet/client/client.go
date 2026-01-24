package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/grace"
	"go-vnet/common/logger"
	"go-vnet/common/session"
	"go-vnet/vnet/client/hub"
	"go-vnet/vnet/server"
)

const (
	ReconnectInterval   = time.Second * 10
	SyncRouterInterval  = time.Second * 5
	MaxErrorToReconnect = 3
)

type (
	Client struct {
		hub      *hub.Hub
		ctx      context.Context
		cfg      *Config
		auth     *auth.Client
		internal internal
		logger   logger.Logger
		manager  *Manager
		session  *session.Session
		sig      chan struct{}
		dev      tun.Tun
	}
	internal interface {
		io.Closer
		hub.TxAdaptor
		Handshake(ctx context.Context, payload map[string]any) (sess *session.Session, sr session.SendReceiveCloser, err error)
		Setup(ctx context.Context) (err error)
	}
	handshakeResponse struct {
		Session *session.Session `json:"session"`
	}
)

func NewClient(cfg *Config) *Client {
	return &Client{
		ctx:    context.Background(),
		cfg:    cfg,
		logger: config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		auth:   auth.NewClient(cfg.Auth),
		sig:    make(chan struct{}),
	}
}

func (c *Client) Run(ctx context.Context) {
	switch c.cfg.Type {
	case TypeQuic:
		c.internal = newQuicClient(c)
	default:
		c.logger.Errorf(ctx, "unsupported client type: %s", c.cfg.Type)
		return
	}

	// run internal client
	if err := c.internal.Setup(ctx); err != nil {
		c.logger.Errorf(ctx, "run %s client error: %v", c.cfg.Type, err.Error())
		return
	}

	// handshake
	sess, sr, err := c.handshake(ctx)
	if err != nil {
		c.logger.Errorf(ctx, "handshake error: %v", err.Error())
		return
	}
	c.session = sess

	// setup tun device
	dev, err := c.setupDevice(ctx, sess)
	if err != nil {
		return
	}
	defer func() {
		if dev != nil {
			_ = dev.Close()
		}
	}()
	c.dev = dev

	// start sync router loop delay
	c.manager = NewManager(sess, sr)
	go func() {
		time.Sleep(time.Millisecond * 500)
		c.syncRouterLoop()
	}()

	// setup hub
	c.hub = hub.NewHub(hub.Config{
		Name:          sess.SessionId,
		MTU:           sess.DispatchedDevice.MTU,
		MaxRxEventBuf: 1024,
		MaxTxEventBuf: 1024,
	}, dev)

	// register grace exit
	grace.Register(ctx, "CloseAndRelease", c.ReleaseAll)

	c.hub.Start()
}

func (c *Client) handshake(ctx context.Context) (session *session.Session, sr session.SendReceiveCloser, err error) {
	payload := c.cfg.authPayload
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["network_id"] = c.cfg.NetworkId

	return c.internal.Handshake(ctx, payload)
}

func (c *Client) syncRouter(ctx context.Context) (err error) {
	// ping
	resp, err := c.manager.CallFunc(ctx, server.FuncNamePing)
	if err != nil {
		c.logger.Errorf(ctx, "failed to execute ping to server: %s", err.Error())
		return
	}
	// c.logger.Infof(ctx, "ping latency: %.2fms", resp.Cost)

	// update router if hash changed
	current := c.hub.Router().MD5()
	if resp.Data == current {
		return
	}

	// get router data from server
	c.logger.Infof(ctx, "router hash changed, current: %s, server: %s", current, resp.Data)
	resp, err = c.manager.CallFunc(ctx, server.FuncNameGetRouterData)
	if err != nil {
		c.logger.Errorf(ctx, "failed to execute get router data from server: %s", err.Error())
		return
	}

	// decode and restore
	if data, ok := resp.Data.(string); ok && len(data) > 0 {
		var bs []byte
		bs, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			c.logger.Errorf(ctx, "failed to decode router data from server: %s", err.Error())
			return
		}

		var ips []string
		err = json.Unmarshal(bs, &ips)
		if err != nil {
			c.logger.Errorf(ctx, "failed to unmarshal router data from server: %s", err.Error())
			return
		}
		mip := map[string]struct{}{}
		for _, ip := range ips {
			mip[ip] = struct{}{}
		}
		cip := map[string]struct{}{}
		for _, ip := range c.hub.Router().Keys() {
			cip[ip] = struct{}{}
		}

		for cidr := range cip {
			_, ok = mip[cidr]
			if !ok {
				c.logger.Infof(ctx, "remove route: %s", cidr)
				ip := strings.Split(cidr, "/")[0]
				v, ok := c.hub.Router().RouteString(ip)
				if ok {
					c.hub.Router().UnRegister(cidr)
				}
				if dst, ok := v.(*hub.Destination); ok {
					_ = dst.Close()
				}
				c.internal.CloseDst(ctx, ip)
			}
		}
		for ip := range mip {
			_, ok = cip[ip]
			if !ok {
				if strings.HasPrefix(ip, c.session.IP) {
					c.hub.Router().Register(ip, nil)
					continue
				}
				c.logger.Infof(ctx, "add route: %s", ip)
				c.hub.Router().Register(ip,
					hub.NewDestination(context.Background(), strings.Split(ip, "/")[0], c.internal, c.hub))
			}
		}

		c.logger.Infof(c.ctx, "router synced from server, data: %d bytes, length: %d",
			len(data), len(c.hub.Router().Keys()))
	}
	return
}

func (c *Client) syncRouterLoop() {
	c.logger.Infof(c.ctx, "sync router loop started")
	defer c.logger.Infof(c.ctx, "sync router loop stopped")

	ticker := time.NewTicker(SyncRouterInterval)
	defer ticker.Stop()

	errCount := 0

	// sync router once
	_ = c.syncRouter(c.ctx)

	for {
		select {
		case <-c.sig:
			c.logger.Infof(c.ctx, "received shutdown signal")
			return
		case <-c.ctx.Done():
			c.logger.Infof(c.ctx, "context cancelled")
			return
		case <-ticker.C:
			// sync router with client context instead of Background
			if err := c.syncRouter(c.ctx); err != nil {
				c.logger.Errorf(c.ctx, "failed to sync router: %s", err.Error())
				errCount++
				if errCount >= MaxErrorToReconnect {
					ticker.Stop()
					c.logger.Infof(c.ctx, "max errors reached (%d), attempting reconnect", errCount)
					go c.reconnectLoop()
					return
				}
			} else {
				// reset error count on success
				errCount = 0
			}
		}
	}
}

func (c *Client) reconnectLoop() {
	var (
		tries = 0
	)

	for {
		c.logger.Infof(c.ctx, "trying to reconnect: %d, in %d seconds", tries+1, int(ReconnectInterval.Seconds()))

		// delay before reconnect to avoid busy loop
		time.Sleep(ReconnectInterval)

		// reconnect
		err := c.Reconnect()
		if err == nil {
			c.logger.Infof(c.ctx, "reconnected successfully")
			return
		}

		c.logger.Errorf(c.ctx, "failed to reconnect: %s", err.Error())
		tries++
	}
}

func (c *Client) Reconnect() (err error) {
	c.logger.Infof(c.ctx, "reconnecting...")
	c.ReleaseAll()
	return errors.New("not implemented")
}

func (c *Client) ReleaseAll() {
	if c.hub != nil {
		c.hub.Stop("client release all")
		c.hub.Router().Range(func(key string, value any) {
			if dst, ok := value.(*hub.Destination); ok {
				_ = dst.Close()
			}
		})
		c.hub = nil
	}
	if c.internal != nil {
		_ = c.internal.Close()
		c.internal = nil
	}
	if c.dev != nil {
		_ = c.dev.Close()
		c.dev = nil
	}
}
