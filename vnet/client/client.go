package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
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
	MaxReconnectInterval = 30
	SyncRouterInterval   = time.Second * 5
	MaxErrorToReconnect  = 3
)

const (
	StateRunning      State = 1
	StateReconnecting State = 2
)

type (
	Client struct {
		state    State
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

		// p2p
		relayInfo *server.RelayInfo
		host      host.Host
		hostId    string
	}
	internal interface {
		io.Closer
		hub.TxAdaptor
		Setup(ctx context.Context) (control session.SendReceiveCloser, err error)
		AfterHandshake(ctx context.Context, session *session.Session)
	}
	handshakeResponse struct {
		Session *session.Session `json:"session"`
	}
	State uint8
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
	if err := c.run(ctx); err != nil {
		c.logger.Errorf(ctx, "run client error: %v", err.Error())
		return
	}

	// register grace exit
	grace.Register(ctx, "CloseAndRelease", c.ReleaseAll)
	// grace exit
	grace.GracefulExit(ctx)
}

func (c *Client) run(ctx context.Context) (err error) {
	// set context
	c.ctx = ctx

	// init internal client
	switch c.cfg.Type {
	case TypeQuic:
		c.internal = newQuicClient(c)
	case TypeTCP:
		c.internal = newTcpClient(c)
	default:
		err = fmt.Errorf("unsupported client type: %s", c.cfg.Type)
		return
	}

	var control session.SendReceiveCloser

	// run internal client
	if control, err = c.internal.Setup(ctx); err != nil {
		c.logger.Errorf(ctx, "run %s client error: %v", c.cfg.Type, err.Error())
		return
	}

	// handshake
	sess, err := c.handshake(ctx, control)
	if err != nil {
		c.logger.Errorf(ctx, "handshake error: %v", err.Error())
		return
	}
	c.session = sess

	// setup tun device
	if c.dev, err = c.setupDevice(ctx, sess); err != nil {
		return
	}

	// create manager for control connection
	c.manager = NewManager(sess, control)

	// setup hub
	c.hub = hub.NewHub(hub.Config{
		Name:          sess.SessionId,
		MTU:           sess.DispatchedDevice.MTU,
		MaxRxEventBuf: 1024,
		MaxTxEventBuf: 1024,
	}, c.dev)

	// start sync router loop delay
	go func() {
		time.Sleep(time.Millisecond * 500)
		c.syncRouterLoop()
	}()

	// start hub
	go c.hub.Start()

	// after hook
	c.internal.AfterHandshake(ctx, sess)

	// setup p2p
	if c.cfg.P2P {
		if err := c.setupP2P(ctx); err != nil {
			c.logger.Errorf(ctx, "failed to setup p2p: %s", err.Error())
		}
	}
	return
}

func (c *Client) handshake(ctx context.Context, sr session.SendReceiveCloser) (session *session.Session, err error) {
	payload := c.cfg.authPayload
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["network_id"] = c.cfg.NetworkId

	resp := &handshakeResponse{}
	err = c.auth.AuthPtr(ctx, payload,
		func(ctx context.Context, in []byte) (out []byte, err error) {
			if err = sr.Send(in); err != nil {
				return
			}
			return sr.Receive(ctx)
		},
		resp,
	)
	session = resp.Session
	c.logger.Infof(ctx, "handshake success: id=%v,ip=%v", session.SessionId, session.IP)
	return
}

func (c *Client) syncRouter(ctx context.Context) (err error) {
	// ping
	resp, err := c.manager.CallFunc(ctx, server.FuncNamePing, map[string]any{
		"host_id": c.hostId,
	})
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
	c.state = StateRunning

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
	c.state = StateReconnecting
	var (
		tries    = 0
		interval = 1
	)

	// 预计算的斐波那契数列（避免递归重复计算）
	fib := func(n int) int {
		if n <= 0 {
			return 1
		}
		a, b := 1, 1
		for i := 2; i <= n; i++ {
			a, b = b, a+b
		}
		return b
	}

	for {
		c.logger.Infof(c.ctx, "trying to reconnect in %d seconds, tries: %d", interval, tries+1)

		// delay before reconnect to avoid busy loop
		time.Sleep(time.Duration(interval) * time.Second)

		// reconnect
		err := c.Reconnect()
		if err == nil {
			c.logger.Infof(c.ctx, "reconnected successfully")
			return
		}

		c.logger.Errorf(c.ctx, "failed to reconnect: %s", err.Error())
		tries++
		interval = fib(tries + 1)
		if interval > MaxReconnectInterval {
			interval = MaxReconnectInterval
		}
	}
}

func (c *Client) Reconnect() (err error) {
	c.logger.Infof(c.ctx, "reconnecting...")
	c.ReleaseAll()
	return c.run(context.Background())
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
	c.manager = nil
	c.session = nil
	c.ctx = nil
	// force GC
	runtime.GC()
}
