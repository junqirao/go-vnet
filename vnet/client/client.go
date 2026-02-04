package client

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/encoding/gbase64"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/util/grand"
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
		p2pSignalingServerAddress *server.AddressInfo
		p2pConnections            sync.Map // dst:*p2pConnInfo
		peerMappingVersion        *atomic.Uint64
		peerMapping               sync.Map
		host                      host.Host
		hostId                    string
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
		ctx:                context.Background(),
		cfg:                cfg,
		logger:             config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger),
		auth:               auth.NewClient(cfg.Auth),
		sig:                make(chan struct{}),
		peerMappingVersion: &atomic.Uint64{},
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
	if c.cfg.P2P.Enabled {
		if err := c.connectP2PSignalingServer(ctx); err != nil {
			c.logger.Errorf(ctx, "failed to setup p2p: %s", err.Error())
		}
	}
	return
}

func (c *Client) handshake(ctx context.Context, sr session.SendReceiveCloser) (session *session.Session, err error) {
	payload := make(map[string]any)

	bs, err := gbase64.DecodeString(c.cfg.Link)
	if err != nil {
		return
	}
	link := server.NetworkLink{}
	if err = json.Unmarshal(bs, &link); err != nil {
		err = fmt.Errorf("invalid link: %w", err)
		return
	}
	link.Nonce = grand.S(8)

	file, err := os.ReadFile(c.cfg.PublicKey)
	if err != nil {
		return
	}
	block, _ := pem.Decode(file)
	if block == nil {
		err = fmt.Errorf("invalid public key")
		return
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		err = fmt.Errorf("invalid public key: %w", err)
		return
	}
	bs, err = rsa.EncryptOAEP(sha256.New(), rand.Reader,
		publicKey.(*rsa.PublicKey), []byte(link.Nonce), []byte("device"))
	if err != nil {
		return
	}
	link.Signature = gbase64.EncodeToString(bs)
	payload["link"] = gbase64.EncodeToString(gjson.MustEncode(link))

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
	if err != nil {
		return
	}
	session = resp.Session
	c.logger.Infof(ctx, "handshake success: id=%v,ip=%v", session.SessionId, session.IP)
	return
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
	c.p2pSignalingServerAddress = nil
	c.p2pConnections.Range(func(key, value any) bool {
		if conn, ok := value.(*p2pConnInfo); ok {
			conn.cancel()
		}
		return true
	})
	c.p2pConnections.Clear()
	c.peerMappingVersion.Store(0)
	c.peerMapping.Clear()
	if c.host != nil {
		_ = c.host.Close()
		c.host = nil
	}
	c.hostId = ""
	c.manager = nil
	c.session = nil
	c.ctx = nil
	// force GC
	runtime.GC()
}
