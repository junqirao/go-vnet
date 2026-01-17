package client

import (
	"context"
	"net/netip"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/vnet/hub"
	"go-vnet/vnet/session"
)

type (
	Client struct {
		hub      *hub.Hub
		cfg      *Config
		auth     *auth.Client
		internal internal
		logger   logger.Logger
	}
	internal interface {
		hub.TxAdaptor
		Handshake(ctx context.Context, payload map[string]any) (sess *session.Session, err error)
		Run(ctx context.Context) (err error)
	}
	handshakeResponse struct {
		Session *session.Session `json:"session"`
	}
)

func NewClient(cfg *Config) *Client {
	return &Client{
		cfg:    cfg,
		logger: config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		auth:   auth.NewClient(cfg.Auth),
	}
}

func (c *Client) Run() {
	// init
	ctx := context.Background()

	switch c.cfg.Type {
	case TypeQuic:
		c.internal = newQuicClient(c)
	default:
		c.logger.Errorf(ctx, "unsupported client type: %s", c.cfg.Type)
		return
	}

	// run internal client
	if err := c.internal.Run(ctx); err != nil {
		c.logger.Errorf(ctx, "run %s client error: %v", c.cfg.Type, err.Error())
		return
	}

	// handshake
	sess, err := c.handshake(ctx)
	if err != nil {
		c.logger.Errorf(ctx, "handshake error: %v", err.Error())
		return
	}

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

	// setup hub
	c.hub = hub.NewHub(hub.Config{
		Name:          "test",
		MTU:           sess.DispatchedDevice.MTU,
		MaxRxEventBuf: 1024,
		MaxTxEventBuf: 1024,
	}, dev)
	c.hub.Start()
}

func (c *Client) handshake(ctx context.Context) (session *session.Session, err error) {
	payload := c.cfg.authPayload
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["network_id"] = c.cfg.NetworkId

	return c.internal.Handshake(ctx, payload)
}

func (c *Client) setupDevice(ctx context.Context, sess *session.Session) (dev tun.Tun, err error) {
	defer func() {
		if err != nil {
			return
		}
		c.logger.Infof(ctx, "setup device success")
	}()

	if sess.DispatchedDevice.Name == "" {
		sess.DispatchedDevice.Name = "tun0"
	}

	c.logger.Infof(ctx, "setup device: name=%s, cidr=%s, mtu=%d, id=%s",
		sess.DispatchedDevice.Name,
		sess.DispatchedDevice.CIDR,
		sess.DispatchedDevice.MTU,
		sess.DispatchedDevice.Id)
	pfx, _ := netip.ParsePrefix(sess.DispatchedDevice.CIDR)
	dev, err = tun.New(tun.Options{
		Name:         sess.DispatchedDevice.Name,
		Inet4Address: []netip.Prefix{pfx},
		Inet6Address: nil,
		MTU:          uint32(sess.DispatchedDevice.MTU),
		GSO:          true,
	})
	if err != nil {
		c.logger.Errorf(ctx, "create tun device error: %v", err.Error())
		return
	}

	// if runtime.GOOS == "linux" {
	// 	var link netlink.Link
	// 	link, err = netlink.LinkByName(sess.DispatchedDevice.Name)
	// 	if err != nil {
	// 		c.logger.Errorf(ctx, "get tun device error: %v", err.Error())
	// 		return
	// 	}
	// 	if err = netlink.LinkSetUp(link); err != nil {
	// 		c.logger.Errorf(ctx, "set tun device up error: %v", err.Error())
	// 		return
	// 	}
	// }
	return
}
