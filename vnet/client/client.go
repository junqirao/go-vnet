package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/encoding/gbase64"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/libp2p/go-libp2p/core/host"
	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/grace"
	"go-vnet/common/metrics"
	"go-vnet/common/protocol"
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
	StateStopped      State = 0
	StateRunning      State = 1
	StateReconnecting State = 2
	StateConnecting   State = 3
)

func (s State) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StateReconnecting:
		return "reconnecting"
	case StateStopped:
		return "stopped"
	case StateConnecting:
		return "connecting"
	default:
		return "unknown"
	}
}

type (
	Client struct {
		id       string
		state    State
		hub      *hub.Hub
		ctx      context.Context
		cfg      *Config
		rc       *session.RequestClient
		internal internal
		manager  *Manager
		session  *session.Session
		sig      chan struct{}
		dev      tun.Tun
		ip       string

		transport struct {
			opts    []protocol.TransportOpt
			metrics *metrics.TransportMetrics
		}

		p2p struct {
			metrics                *metrics.TransportMetrics
			signalingServerAddress *server.AddressInfo
			connections            sync.Map // dst:*p2pConnInfo
			peerMappingVersion     *atomic.Uint64
			peerMapping            sync.Map
			host                   host.Host
			hostId                 string
		}
	}
	internal interface {
		io.Closer
		hub.TxAdaptor
		Setup(ctx context.Context) (control session.SendReceiveCloser, err error)
		AfterHandshake(ctx context.Context, session *session.Session)
	}
	handshakeResponse struct {
		Session *session.Session `json:"session"`
		Error   string           `json:"error"`
	}
	State       uint8
	RuntimeInfo struct {
		State       string                    `json:"state"`
		Session     *session.Session          `json:"session"`
		Metrics     *metrics.TransportMetrics `json:"metrics"`
		Router      *RouterRuntimeInfo        `json:"router"`
		P2P         *P2PRuntimeInfo           `json:"p2p"`
		Connections []*ConnectionInfo         `json:"connections"`
		Config      *Config                   `json:"config"`
	}
	RouterRuntimeInfo struct {
		Routers []string `json:"routers"`
		Version string   `json:"version"`
	}
	P2PRuntimeInfo struct {
		HostId         string                    `json:"host_id"`
		MappingVersion uint64                    `json:"mapping_version"`
		Connections    []string                  `json:"connections"`
		Metrics        *metrics.TransportMetrics `json:"metrics"`
	}
	ConnectionInfo struct {
		Dst  string `json:"dst"`
		Type string `json:"type"`
	}
)

func NewClient(cfg *Config) *Client {
	pk, err := os.ReadFile(cfg.PublicKey)
	if err != nil {
		panic(err)
	}

	rc, err := session.NewRequestClient(string(pk))
	if err != nil {
		panic(err)
	}

	opts := []protocol.TransportOpt{
		protocol.EnableEncrypt(cfg.Encrypt),
		protocol.EnableCompress(cfg.Compress),
	}

	if cfg.Compress {
		compressor, err := protocol.NewZstdCompressor(protocol.FastestCompressionLevel)
		if err != nil {
			panic(err)
		}
		opts = append(opts, protocol.WithCompressor(compressor))
	}

	c := &Client{
		ctx: context.Background(),
		cfg: cfg,
		rc:  rc,
		sig: make(chan struct{}),
	}

	// create once and run non-stop update loop
	c.transport.metrics = metrics.NewTransportMetrics()
	go c.nonStopUpdateMetricsLoop()
	c.transport.opts = opts
	c.p2p.peerMappingVersion = &atomic.Uint64{}
	return c
}

func (c *Client) Run(ctx context.Context) {
	c.state = StateConnecting
	if err := c.run(ctx); err != nil {
		// wait 100ms make sure client is under reconnecting state or not
		time.Sleep(time.Millisecond * 100)
		if c.state != StateReconnecting {
			c.state = StateStopped
		}
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
		g.Log().Errorf(ctx, "run %s client error: %v", c.cfg.Type, err.Error())
		// already under retry loop do not make new loop
		if v := ctx.Value("retry"); v != nil && v.(bool) {
			return
		}
		// mostly is network error, retry
		go c.reconnectLoop()
		// reset error
		err = nil
		return
	}
	g.Log().Infof(ctx, "connect success")

	// handshake
	sess, err := c.handshake(ctx, control)
	if err != nil {
		g.Log().Errorf(ctx, "handshake error: %v", err.Error())
		return
	}
	c.session = sess

	// setup tun device
	if c.dev, err = c.setupDevice(ctx, sess); err != nil {
		return
	}

	// setup encryptor
	encryptor, err := protocol.NewChacha20Poly1305Encryptor([]byte(sess.DispatchedDevice.Key)[:32])
	if err != nil {
		g.Log().Errorf(ctx, "failed to setup encryptor: %s", err.Error())
		return
	}

	// transport options
	c.transport.opts = append(c.transport.opts,
		protocol.WithMetrics(c.transport.metrics),
		protocol.WithEncryptor(encryptor))

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
		c.p2p.metrics = metrics.NewTransportMetrics()
		if err := c.connectP2PSignalingServer(ctx); err != nil {
			g.Log().Errorf(ctx, "failed to setup p2p: %s", err.Error())
		}
	}
	return
}

func (c *Client) handshake(ctx context.Context, sr session.SendReceiveCloser) (ss *session.Session, err error) {
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

	payload["hostname"], _ = os.Hostname()
	payload["encrypt"] = c.cfg.Encrypt
	payload["compress"] = c.cfg.Compress
	payload["p2p"] = c.cfg.P2P.Enabled
	payload["session"] = c.id

	resp := &handshakeResponse{}
	if err = c.rc.Do(ctx, sr, session.NewHeader(link.SubDeviceId, link.Key), payload, resp); err != nil {
		return
	}
	if resp.Error != "" {
		err = errors.New(resp.Error)
		return
	}
	ss = resp.Session
	c.id = resp.Session.SessionId
	g.Log().Infof(ctx, "handshake success: id=%v,ip=%v", ss.SessionId, ss.IP)
	return
}

func (c *Client) Reconnect(ctx context.Context) (err error) {
	g.Log().Infof(c.ctx, "reconnecting...")
	c.ReleaseAll()
	return c.run(ctx)
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
	c.p2p.signalingServerAddress = nil
	c.p2p.connections.Range(func(key, value any) bool {
		if conn, ok := value.(*p2pConnInfo); ok {
			conn.cancel()
		}
		return true
	})
	c.p2p.connections.Clear()
	c.p2p.peerMappingVersion.Store(0)
	c.p2p.peerMapping.Clear()
	if c.p2p.host != nil {
		_ = c.p2p.host.Close()
		c.p2p.host = nil
	}
	c.p2p.hostId = ""
	if c.manager != nil {
		c.manager.Close()
	}
	c.manager = nil
	c.session = nil
	c.ctx = nil
	// force GC
	runtime.GC()
}

func (c *Client) nonStopUpdateMetricsLoop() {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			if c.transport.metrics != nil {
				c.transport.metrics.UpdateAll()
			}
			if c.p2p.metrics != nil {
				c.p2p.metrics.UpdateAll()
			}
		}
	}
}

func (c *Client) CollectRuntimeInfo() (info *RuntimeInfo) {
	info = &RuntimeInfo{
		State:   c.state.String(),
		Config:  c.cfg,
		Metrics: c.transport.metrics,
	}
	if c.state != StateRunning {
		return
	}
	info.Session = c.session
	info.P2P = &P2PRuntimeInfo{
		HostId:         c.p2p.hostId,
		MappingVersion: c.p2p.peerMappingVersion.Load(),
		Metrics:        c.p2p.metrics,
		Connections:    []string{},
	}
	c.p2p.connections.Range(func(key, value any) bool {
		info.P2P.Connections = append(info.P2P.Connections, key.(*hub.Destination).Ip())
		return true
	})
	info.Metrics = c.transport.metrics
	info.Router = &RouterRuntimeInfo{}
	router := c.hub.Router()
	info.Router.Version = router.MD5()
	router.Range(func(addr string, val any) {
		if dst, ok := val.(*hub.Destination); ok {
			info.Router.Routers = append(info.Router.Routers, dst.Ip())
			info.Connections = append(info.Connections, &ConnectionInfo{
				Dst:  dst.Ip(),
				Type: dst.Type(),
			})
		}
	})
	slices.SortFunc(info.Router.Routers, func(a, b string) int {
		return strings.Compare(a, b)
	})
	slices.SortFunc(info.Connections, func(a, b *ConnectionInfo) int {
		return strings.Compare(a.Dst, b.Dst)
	})
	return
}

func (c *Client) Stop(ctx context.Context) (err error) {
	g.Log().Infof(ctx, "stopping client...")
	c.state = StateStopped
	c.ReleaseAll()
	return
}

func (c *Client) Resume(ctx context.Context) (err error) {
	g.Log().Infof(ctx, "resuming client...")
	// set state to reconnecting to avoid reconnecting loop
	// break on stopped state
	c.state = StateReconnecting
	return c.Reconnect(ctx)
}
