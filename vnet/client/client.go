package client

import (
	"context"
	"fmt"
	"net/netip"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/logger"
	"go-vnet/vnet/hub"
)

type (
	Client struct {
		hub      *hub.Hub
		cfg      *Config
		internal internal
		logger   logger.Logger
	}
	internal interface {
		hub.TxAdaptor
		Run(ctx context.Context) (err error)
	}
)

func NewClient(cfg *Config) *Client {
	return &Client{
		cfg:    cfg,
		logger: logger.DefaultLogger,
	}
}

func (c *Client) Run() {
	var (
		err error
		ip  = "192.168.99.2"
		mtu = 1392
	)

	pfx, _ := netip.ParsePrefix(fmt.Sprintf("%s/24", ip))

	dev, err := tun.New(tun.Options{
		Name:         "tun0",
		Inet4Address: []netip.Prefix{pfx},
		Inet6Address: nil,
		MTU:          uint32(mtu),
	})
	if err != nil {
		panic(err)
		return
	}
	c.hub = hub.NewHub(hub.Config{
		Name:          "test",
		MTU:           mtu,
		MaxRxEventBuf: 1024,
		MaxTxEventBuf: 1024,
	}, dev)

	switch c.cfg.Type {
	case TypeQuic:
		c.internal = newQuicClient(c)
	default:
		panic(fmt.Errorf("unsupported client type: %s", c.cfg.Type))
	}

	ctx := context.Background()
	if err = c.internal.Run(ctx); err != nil {
		panic(fmt.Errorf("run %s client error: %w", c.cfg.Type, err))
	}

	c.hub.Start()
}
