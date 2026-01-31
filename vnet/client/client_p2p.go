package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
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

func (c *Client) connectP2PSignalingServer(ctx context.Context) (err error) {
	c.p2pSignalingServerAddress, err = c.getPeerInfo(ctx)
	if err != nil {
		return
	}

	if len(c.p2pSignalingServerAddress.Addresses) == 0 {
		c.logger.Infof(ctx, "no p2p signaling server address available")
		return
	}

	if c.p2pSignalingServerAddress == nil || c.p2pSignalingServerAddress.Id == "" {
		return
	}
	var (
		address []multiaddr.Multiaddr
		opts    = []libp2p.Option{
			libp2p.EnableNATService(),
			libp2p.EnableRelay(),
			libp2p.EnableHolePunching(),
		}
		localListenAddr []string
	)

	localListenAddr = append(localListenAddr, c.cfg.P2P.ListenAddr...)
	if len(localListenAddr) == 0 {
		localListenAddr = append(localListenAddr, "/ip4/0.0.0.0/tcp/0")
	}

	for _, s := range localListenAddr {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			c.logger.Errorf(ctx, "invalid p2p listen address: %s, reason: %v", s, err)
			continue
		}
		address = append(address, ma)
	}

	opts = append(opts, libp2p.ListenAddrs(address...))

	c.host, err = libp2p.New(opts...)
	if err != nil {
		return
	}
	c.host.SetStreamHandler(p2pProtocolID, c.handleP2PStreamRx)

	serverAddrInfo := &peer.AddrInfo{}
	serverAddrInfo.ID, _ = peer.Decode(c.p2pSignalingServerAddress.Id)
	for _, s := range c.p2pSignalingServerAddress.Addresses {
		ma, err := multiaddr.NewMultiaddr(fmt.Sprintf("%s/p2p/%s", s, c.p2pSignalingServerAddress.Id))
		if err != nil {
			c.logger.Errorf(ctx, "invalid p2p signaling server address: %s, reason: %v", s, err)
			continue
		}
		serverAddrInfo.Addrs = append(serverAddrInfo.Addrs, ma)
	}

	c.logger.Infof(ctx, "connecting to p2p signaling server: %s/p2p/%s", serverAddrInfo.String(), serverAddrInfo.ID)

	err = c.host.Connect(c.ctx, *serverAddrInfo)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}

	c.hostId = c.host.ID().String()
	peerInfo := peer.AddrInfo{
		ID:    c.host.ID(),
		Addrs: c.host.Addrs(),
	}
	peerInfoStr, _ := peerInfo.MarshalJSON()
	// register p2p peer
	_, err = c.manager.CallFunc(ctx, server.FuncNameRegisterP2PPeer, map[string]any{
		"peer": string(peerInfoStr),
	})
	if err != nil {
		return fmt.Errorf("failed to register p2p peer: %v", err)
	}
	c.logger.Infof(ctx, "connected to p2p signaling server, local peer info: %s", peerInfoStr)
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
	c.logger.Infof(ctx, "accept p2p stream from %s", stream.Conn().RemotePeer())
	err := c.evaluateAndReplaceP2PRx(ctx, stream)
	if err != nil {
		c.logger.Infof(ctx, "evaluate p2p stream failed: %s", err.Error())
		return
	}
}

func (c *Client) dialDstRelay(ctx context.Context, dst string) (stream network.Stream, err error) {
	v, ok := c.peerMapping.Load(dst)
	if !ok {
		return nil, errors.New("destination didnt register p2p")
	}
	targetPeer := &peer.AddrInfo{}
	if err = json.Unmarshal([]byte(v.(string)), targetPeer); err != nil {
		err = fmt.Errorf("failed to parse p2p address: %v", err)
		return
	}
	c.logger.Infof(ctx, "dialing p2p stream to %s: %s", dst, targetPeer.String())
	if err = c.host.Connect(c.ctx, *targetPeer); err != nil {
		err = fmt.Errorf("failed to connect to peer: %v", err)
		return
	}
	c.logger.Infof(ctx, "successfully connected to peer: %s", targetPeer.ID.ShortString())
	stream, err = c.host.NewStream(c.ctx, targetPeer.ID, p2pProtocolID)
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
	c.logger.Infof(ctx, "send handshake %v to dst %s, waiting for ack...", pkg, dst.Ip())
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
	dst.ReplaceWithFallback(protocol.NewTransport(stream), func(rw protocol.ReadWriter, err error) {
		_ = rw.Close()
		c.logger.Infof(ctx, "dst %s connection fallback to proxy: %s", dst.Ip(), err.Error())
	})
	c.logger.Infof(ctx, "replace dst %s to p2p connection", dst.Ip())
	c.hub.HandleRx(stream)
	c.logger.Infof(ctx, "dst %s p2p connection rx started", dst.Ip())
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
		n     int
		ipBuf = make([]byte, 4)
	)

	if n, err = stream.Read(ipBuf); err != nil {
		return
	}
	dst := net.IPv4(ipBuf[0], ipBuf[1], ipBuf[2], ipBuf[3]).To4().String()
	c.logger.Infof(ctx, "receive handshake %v from dst %s", ipBuf[:n], dst)
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

	c.logger.Infof(ctx, "evaluation success %s, try replace", dst)
	v, ok := c.hub.Router().RouteString(dst)
	if ok {
		if d, ok := v.(*hub.Destination); ok {
			c.logger.Infof(ctx, "replace dst %s to p2p connection", dst)
			d.ReplaceWithFallback(protocol.NewTransport(stream), func(rw protocol.ReadWriter, err error) {
				_ = rw.Close()
				c.logger.Infof(ctx, "dst %s connection fallback to proxy: %s", dst, err.Error())
			})
		}
	}
	c.hub.HandleRx(stream)
	c.logger.Infof(ctx, "dst %s p2p connection rx started", dst)
	return
}

func (c *Client) tryP2P(ctx context.Context, dst *hub.Destination) {
	c.logger.Infof(ctx, "try connect and evaluate p2p tx: %s", dst.Ip())
	_ = c.workerPool.Submit(func() {
		stream, err := c.dialDstRelay(ctx, dst.Ip())
		if err != nil {
			c.logger.Infof(ctx, "dial p2p peer stream failed: %s", err.Error())
			return
		}

		c.logger.Infof(ctx, "dial p2p stream success, start evaluate p2p tx: %s", dst.Ip())
		if err = c.evaluateAndReplaceP2PTx(ctx, dst, stream); err != nil {
			c.logger.Infof(ctx, "evaluate p2p tx failed: %s", err.Error())
			return
		}
		c.logger.Infof(ctx, "evaluate p2p tx success: %s", dst.Ip())
		return
	})
}

func (c *Client) AfterDial(ctx context.Context, dst *hub.Destination) {
	if !c.cfg.P2P.Enabled {
		return
	}
	c.tryP2P(ctx, dst)
}

func (c *Client) OnFallback(ctx context.Context, dst *hub.Destination, rw protocol.ReadWriter, err error) {
	c.logger.Infof(ctx, "dst %s connection fallback to proxy: %s", dst.Ip(), err.Error())
	_ = rw.Close()
	if !c.cfg.P2P.Enabled {
		return
	}
	c.logger.Infof(ctx, "try connect and evaluate p2p in 30s: %s", dst.Ip())
	_ = c.workerPool.Submit(func() {
		time.Sleep(time.Second * 30)
		c.tryP2P(ctx, dst)
	})
}
