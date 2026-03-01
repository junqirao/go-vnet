package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/multiformats/go-multiaddr"

	"go-vnet/common/protocol"
	"go-vnet/vnet/client/hub"
	"go-vnet/vnet/server"
)

const (
	acceptableLatency = 1000
	p2pProtocolID     = "/p2p/transport/1.0.0"
)

var (
	evaluateFail = [1]byte{1}
	evaluatePass = [1]byte{2}
)

type (
	p2pConnInfo struct {
		stream network.Stream
		cancel func()
	}
	p2pHandleRxHook struct {
		c   *Client
		dst string
	}
)

func (p *p2pHandleRxHook) OnStart() {
	g.Log().Infof(p.c.ctx, "p2p handle rx %s started", p.dst)
}

func (p *p2pHandleRxHook) OnClose(err error) {
	g.Log().Infof(p.c.ctx, "p2p handle rx %s closed: %v", p.dst, err)
}

func (c *Client) connectP2PSignalingServer(ctx context.Context) (err error) {
	c.p2p.signalingServerAddress, err = c.getPeerInfo(ctx)
	if err != nil {
		return
	}

	if len(c.p2p.signalingServerAddress.Addresses) == 0 {
		g.Log().Infof(ctx, "no p2p signaling server address available")
		return
	}

	if c.p2p.signalingServerAddress == nil || c.p2p.signalingServerAddress.Id == "" {
		return
	}
	var (
		address []multiaddr.Multiaddr
		opts    = []libp2p.Option{
			libp2p.EnableNATService(),
			libp2p.EnableRelay(),
			libp2p.EnableHolePunching(),
			libp2p.EnableAutoNATv2(),
			libp2p.NATPortMap(),
		}
		localListenAddr []string
	)

	localListenAddr = append(localListenAddr, c.cfg.P2P.ListenAddr...)
	if len(localListenAddr) == 0 {
		// 默认同时监听 TCP 和 UDP
		localListenAddr = append(localListenAddr, "/ip4/0.0.0.0/tcp/0")
		localListenAddr = append(localListenAddr, "/ip4/0.0.0.0/udp/0/quic-v1")
	}

	for _, s := range localListenAddr {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			g.Log().Errorf(ctx, "invalid p2p listen address: %s, reason: %v", s, err)
			continue
		}
		address = append(address, ma)
	}

	// 配置地址工厂函数：过滤掉回环地址，保留可观察到的地址
	opts = append(opts, libp2p.AddrsFactory(func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
		var result []multiaddr.Multiaddr
		for _, addr := range addrs {
			s := addr.String()
			// 过滤掉回环地址和未指定地址
			if strings.Contains(s, "/ip4/127.0.0.1") || strings.Contains(s, "/ip6/::1") {
				continue
			}
			// 保留公网可达的地址，包括TCP和UDP
			result = append(result, addr)
		}
		// 增加地址数量限制以包含UDP地址
		if len(result) > 6 {
			result = result[:6]
		}
		return result
	}))

	// 配置连接管理器：优化用于混合TCP/UDP环境
	c.p2p.connMgr, err = connmgr.NewConnManager(
		30,                                        // 低水位
		150,                                       // 高水位：支持更多连接类型
		connmgr.WithGracePeriod(time.Minute*2),    // 优雅期
		connmgr.WithSilencePeriod(time.Second*20), // 延长静默期
	)
	if err != nil {
		return fmt.Errorf("failed to create connection manager: %v", err)
	}

	opts = append(opts, libp2p.ListenAddrs(address...))
	opts = append(opts, libp2p.ConnectionManager(c.p2p.connMgr))

	c.p2p.host, err = libp2p.New(opts...)
	if err != nil {
		return
	}
	c.p2p.host.SetStreamHandler(p2pProtocolID, c.handleP2PStreamRx)

	serverAddrInfo := &peer.AddrInfo{}
	serverAddrInfo.ID, _ = peer.Decode(c.p2p.signalingServerAddress.Id)
	for _, s := range c.p2p.signalingServerAddress.Addresses {
		ma, addrErr := multiaddr.NewMultiaddr(fmt.Sprintf("%s/p2p/%s", s, c.p2p.signalingServerAddress.Id))
		if addrErr != nil {
			g.Log().Errorf(ctx, "invalid p2p signaling server address: %s, reason: %v", s, addrErr)
			continue
		}
		serverAddrInfo.Addrs = append(serverAddrInfo.Addrs, ma)
	}

	g.Log().Infof(ctx, "connecting to p2p signaling server: %s", serverAddrInfo.ID.ShortString())

	g.Log().Infof(ctx, "attempting to connect to signaling server with %d addresses", len(serverAddrInfo.Addrs))
	for i, addr := range serverAddrInfo.Addrs {
		g.Log().Debugf(ctx, "address %d: %s", i, addr.String())
	}

	err = c.p2p.host.Connect(c.ctx, *serverAddrInfo)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}

	c.p2p.hostId = c.p2p.host.ID().String()
	peerInfo := peer.AddrInfo{
		ID:    c.p2p.host.ID(),
		Addrs: c.p2p.host.Addrs(),
	}
	peerInfoStr, _ := peerInfo.MarshalJSON()
	// register p2p peer
	_, err = c.manager.CallFunc(ctx, server.FuncNameRegisterP2PPeer, map[string]any{
		"peer": string(peerInfoStr),
	})
	if err != nil {
		return fmt.Errorf("failed to register p2p peer: %v", err)
	}
	g.Log().Info(ctx, "successfully connected to p2p signaling server")
	c.backgroundTryDialP2P()
	// 启动定期刷新地址的任务，确保NAT映射保持活跃
	c.startPeriodicAddressRefresh()
	return
}

func (c *Client) getPeerInfo(ctx context.Context) (info *server.AddressInfo, err error) {
	resp, err := c.manager.CallFunc(ctx, server.FuncNameGetP2PPeerInfo)
	if err != nil {
		return
	}
	info = new(server.AddressInfo)
	data, err := base64.StdEncoding.DecodeString(resp.Data.(string))
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &info)
	return
}

func (c *Client) handleP2PStreamRx(stream network.Stream) {
	ctx := c.ctx
	exist := false
	c.p2p.router.Range(func(key string, value any) {
		pi := value.(*peer.AddrInfo)
		if pi.ID == stream.Conn().RemotePeer() {
			exist = true
		}
	})

	if !exist {
		_ = stream.Close()
		g.Log().Infof(ctx, "p2p stream from unknown peer: %s", stream.Conn().RemotePeer())
		return
	}

	g.Log().Infof(ctx, "accept p2p stream from %s", stream.Conn().RemotePeer())
	err := c.evaluateAndReplaceP2PRx(ctx, stream)
	if err != nil {
		g.Log().Infof(ctx, "evaluate p2p stream failed: %s", err.Error())
		return
	}
}

func (c *Client) dialDstRelay(ctx context.Context, targetPeer *peer.AddrInfo, dst string) (stream network.Stream, err error) {
	g.Log().Infof(ctx, "dialing p2p stream to %s: %s", dst, targetPeer.String())
	if err = c.p2p.host.Connect(c.ctx, *targetPeer); err != nil {
		err = fmt.Errorf("failed to connect to peer: %v", err)
		return
	}
	g.Log().Infof(ctx, "successfully connected to peer: %s", targetPeer.ID.ShortString())
	stream, err = c.p2p.host.NewStream(c.ctx, targetPeer.ID, p2pProtocolID)
	return
}

func (c *Client) evaluateAndReplaceP2PTx(ctx context.Context, dst *hub.Destination, stream network.Stream) (err error) {
	// ping and wait result
	start := time.Now()
	_ = stream.SetWriteDeadline(start.Add(time.Second * 2))
	_ = stream.SetReadDeadline(start.Add(time.Second * 2))
	defer func() {
		_ = stream.SetWriteDeadline(time.Time{})
		_ = stream.SetReadDeadline(time.Time{})
	}()
	ip := net.ParseIP(c.session.IP)
	pkg := []byte{ip[12], ip[13], ip[14], ip[15]}
	if _, err = stream.Write(pkg); err != nil {
		return
	}
	if _, err = stream.Read(make([]byte, 1)); err != nil {
		return
	}
	cost := time.Since(start).Milliseconds()
	ok := cost < acceptableLatency
	if !ok {
		// ack
		_, _ = stream.Write(evaluateFail[:])
		_ = stream.Close()
		err = fmt.Errorf("cannot accept latency of : %vms>%vms", cost, acceptableLatency)
		return
	}
	_, _ = stream.Write(evaluatePass[:])
	c.replaceP2P(ctx, stream, dst)
	return
}

func (c *Client) evaluateAndReplaceP2PRx(ctx context.Context, stream network.Stream) (err error) {
	// ping and wait result
	start := time.Now()
	_ = stream.SetWriteDeadline(start.Add(time.Second * 2))
	_ = stream.SetReadDeadline(start.Add(time.Second * 2))
	defer func() {
		_ = stream.SetWriteDeadline(time.Time{})
		_ = stream.SetReadDeadline(time.Time{})
	}()
	var (
		ipBuf = make([]byte, 4)
	)

	if _, err = stream.Read(ipBuf); err != nil {
		return
	}
	dst := net.IPv4(ipBuf[0], ipBuf[1], ipBuf[2], ipBuf[3]).To4().String()
	if _, err = stream.Write([]byte{0}); err != nil {
		return
	}
	buf := make([]byte, 1)
	if _, err = stream.Read(buf); err != nil {
		return
	}
	if buf[0] == evaluateFail[0] {
		_ = stream.Close()
		err = errors.New("stream closed by peer")
		return
	}
	if buf[0] != evaluatePass[0] {
		err = errors.New("internal error: invalid ack packet")
		return
	}

	v, ok := c.hub.Router().RouteString(dst)
	if ok {
		if d, ok := v.(*hub.Destination); ok {
			c.replaceP2P(ctx, stream, d)
		}
	}
	return
}

func (c *Client) replaceP2P(ctx context.Context, stream network.Stream, dst *hub.Destination) {
	metrics := protocol.WithWrappers(func(upstream io.ReadWriteCloser) io.ReadWriteCloser {
		return protocol.NewMetricsWrapper(stream, c.p2p.metrics)
	})
	transportOptions := append(c.transport.opts, metrics, protocol.WithType(protocol.TransportTypeP2P))
	t := protocol.NewTransport(stream, transportOptions...)
	cancel := c.hub.HandleRx(stream, transportOptions, &p2pHandleRxHook{c: c, dst: dst.Ip()})
	cancelAll := func() {
		cancel()
		_ = t.Close()
	}
	// replace
	replaced := dst.Replace(func(old protocol.ReadWriter) (new protocol.ReadWriter, replaced bool) {
		if old != nil && old.Type() == protocol.TransportTypeP2P {
			return
		}
		new = t
		replaced = true
		return
	})
	if !replaced {
		cancelAll()
		return
	}
	g.Log().Infof(ctx, "replace dst %s to p2p connection", dst.Ip())
	// register
	c.p2p.connections.Store(dst, &p2pConnInfo{
		stream: stream,
		cancel: cancelAll,
	})
}

func (c *Client) tryP2P(ctx context.Context, dst *hub.Destination) (err error) {
	if !c.cfg.P2P.Enabled {
		g.Log().Infof(ctx, "p2p is not enabled, skip dial p2p to %s", dst.Ip())
		err = hub.P2PNotEnabled
		return
	}
	if hub.DeviceModeFromString(c.cfg.DeviceMode) == hub.DeviceModeProxyOnly {
		g.Log().Infof(ctx, "device mode is under proxy only, skip dial p2p to %s", dst.Ip())
		err = hub.P2PNotEnabled
		return
	}
	if dst.Type() == protocol.TransportTypeP2P {
		return
	}

	v, ok := c.p2p.router.RouteString(dst.Ip())
	if !ok {
		return
	}
	g.Log().Infof(ctx, "try connect and evaluate p2p tx: %s", dst.Ip())
	stream, err := c.dialDstRelay(ctx, v.(*peer.AddrInfo), dst.Ip())
	if err != nil {
		g.Log().Infof(ctx, "dial p2p peer stream failed: %s", err.Error())
		return
	}

	g.Log().Infof(ctx, "dial p2p stream success, start evaluate p2p tx: %s", dst.Ip())
	if err = c.evaluateAndReplaceP2PTx(ctx, dst, stream); err != nil {
		g.Log().Infof(ctx, "evaluate p2p tx failed: %s", err.Error())
		return
	}
	g.Log().Infof(ctx, "evaluate p2p tx success: %s", dst.Ip())
	return
}

func (c *Client) backgroundTryDialP2P() {
	if !c.cfg.P2P.Enabled || !c.cfg.P2P.ActiveDialPeer {
		return
	}
	g.Log().Infof(c.ctx, "background try dial p2p started")
	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-c.sig:
			default:
			}
			c.hub.Router().Range(func(addr string, val any) {
				dst, ok := val.(*hub.Destination)
				if !ok {
					return
				}
				err := c.tryP2P(c.ctx, dst)
				if err != nil {
					g.Log().Infof(c.ctx, "dst %s try p2p failed: %s", dst.Ip(), err.Error())
				}
			})
			time.Sleep(time.Duration(c.cfg.P2P.TryInterval) * time.Second)
		}
	}()
}

func (c *Client) AfterDial(ctx context.Context, dst *hub.Destination) {
	err := c.tryP2P(ctx, dst)
	if err != nil {
		g.Log().Infof(ctx, "dst %s try p2p failed: %s", dst.Ip(), err.Error())
	}
}

func (c *Client) OnFallback(ctx context.Context, dst *hub.Destination, rw protocol.ReadWriter, err error) {
	g.Log().Infof(ctx, "dst %s connection fallback to %s: %s", dst.Ip(), dst.Type(), err.Error())
	if rw.Type() == protocol.TransportTypeP2P {
		v, loaded := c.p2p.connections.LoadAndDelete(dst.Ip())
		if loaded {
			info := v.(*p2pConnInfo)
			info.cancel()
		}
	}
}

// startPeriodicAddressRefresh 启动定期刷新NAT地址的任务
// 这样可以确保NAT映射保持活跃，新客户端可以连接到现有的P2P网络
func (c *Client) startPeriodicAddressRefresh() {
	if !c.cfg.P2P.Enabled {
		return
	}

	// 从配置获取刷新间隔，默认120秒（2分钟）
	refreshInterval := time.Second * time.Duration(c.cfg.P2P.AddressRefresh)
	if refreshInterval < time.Second*30 {
		refreshInterval = time.Second * 30 // 最小30秒
	}

	g.Log().Infof(c.ctx, "starting periodic p2p address refresh (interval: %v)", refreshInterval)

	go func() {
		ticker := time.NewTicker(refreshInterval)
		defer ticker.Stop()

		// 首次延迟30秒开始刷新，避免启动时立即刷新
		time.Sleep(time.Second * 30)

		for {
			select {
			case <-c.ctx.Done():
				g.Log().Infof(c.ctx, "periodic p2p address refresh stopped")
				return
			case <-c.sig:
				g.Log().Infof(c.ctx, "periodic p2p address refresh stopped by signal")
				return
			case <-ticker.C:
				c.refreshP2PAddress()
			}
		}
	}()
}

// refreshP2PAddress 向服务器请求刷新自己的P2P地址
func (c *Client) refreshP2PAddress() {
	if c.manager == nil {
		return
	}

	ctx, cancel := context.WithTimeout(c.ctx, time.Second*10)
	defer cancel()

	g.Log().Debugf(ctx, "refreshing p2p address...")

	resp, err := c.manager.CallFunc(ctx, server.FuncNameRefreshP2PAddress)
	if err != nil {
		g.Log().Errorf(ctx, "failed to refresh p2p address: %v", err)
		return
	}

	if resp.Code != 0 {
		g.Log().Errorf(ctx, "refresh p2p address returned error code: %d", resp.Code)
		return
	}

	g.Log().Debugf(ctx, "p2p address refreshed successfully")
}
