package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"

	"go-vnet/common/protocol"
	"go-vnet/vnet/client/hub"
	"go-vnet/vnet/server"
)

const (
	acceptableLatency = 1000
)

var (
	evaluateFail = [1]byte{1}
	evaluatePass = [1]byte{2}
)

func (c *Client) setupP2P(ctx context.Context) (err error) {
	c.relayInfo, err = c.getRelayInfo(ctx)
	if err != nil {
		return
	}

	if len(c.relayInfo.Addresses) == 0 {
		return
	}

	var (
		addresses = sync.Map{}
		address   string
		wg        = sync.WaitGroup{}
	)
	wg.Add(len(c.relayInfo.Addresses))

	for _, addr := range c.relayInfo.Addresses {
		_ = c.workerPool.Submit(func() {
			defer wg.Done()
			c.logger.Infof(ctx, "test p2p relay address: %s", addr)
			ok, cost, err := checkMultiAddrConnectivity(addr, time.Second*1)
			if err != nil || !ok {
				c.logger.Infof(ctx, "test p2p relay address failed: %s, reason:%v", addr, err)
				return
			}
			c.logger.Infof(ctx, "test p2p relay address success: %s, cost: %dms", addr, cost)
			addresses.Store(addr, cost)
		})
	}

	wg.Wait()
	minCost := int64(math.MaxInt64)
	addresses.Range(func(key, value any) bool {
		addr := key.(string)
		cost := value.(int64)
		if cost < minCost {
			address = addr
			minCost = cost
		}
		return true
	})

	if address == "" {
		c.logger.Infof(ctx, "no p2p relay address available")
		return
	}

	c.logger.Infof(ctx, "selected p2p relay address: %s, cost: %dms", address, minCost)
	if err = c.registerRelay(ctx, address); err != nil {
		return
	}
	return
}

func checkMultiAddrConnectivity(addrStr string, timeout time.Duration) (ok bool, cost int64, err error) {
	start := time.Now()
	defer func() {
		cost = time.Since(start).Milliseconds()
	}()
	// 解析 Multiaddr
	addr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		err = fmt.Errorf("invalid multiaddr: %w", err)
		return
	}

	// 转换为标准网络地址
	naddr, err := manet.ToNetAddr(addr)
	if err != nil {
		err = fmt.Errorf("unsupported protocol: %w", err)
		return
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 尝试建立连接
	done := make(chan bool, 1)
	go func() {
		conn, err := net.Dial(naddr.Network(), naddr.String())
		if err == nil {
			conn.Close()
			done <- true
			return
		}
		done <- false
	}()

	select {
	case success := <-done:
		ok = success
		return
	case <-ctx.Done():
		err = ctx.Err()
		return
	}
}

func (c *Client) getRelayInfo(ctx context.Context) (info *server.RelayInfo, err error) {
	resp, err := c.manager.CallFunc(ctx, server.FuncNameGetP2PRelayInfo)
	if err != nil {
		return
	}
	info = new(server.RelayInfo)
	data, err := base64.StdEncoding.DecodeString(resp.Data.(string))
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &info)
	return
}

func (c *Client) registerRelay(ctx context.Context, addr string) (err error) {
	if c.relayInfo == nil || c.relayInfo.Id == "" {
		return
	}
	host, err := libp2p.New(
		libp2p.NoListenAddrs,
		libp2p.EnableRelay(),
	)
	if err != nil {
		return
	}
	c.host = host

	relayAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("%s/p2p/%s", addr, c.relayInfo.Id))
	if err != nil {
		return
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(relayAddr)
	if err != nil {
		return
	}

	if err = host.Connect(ctx, *addrInfo); err != nil {
		return
	}

	// Add relay addresses to peerstore for future use
	host.Peerstore().AddAddrs(addrInfo.ID, addrInfo.Addrs, time.Hour*24)

	_, err = client.Reserve(ctx, host, *addrInfo)
	if err != nil {
		return
	}

	host.SetStreamHandler("/transport", func(stream network.Stream) {
		c.logger.Infof(ctx, "accept p2p stream from %s", stream.Conn().RemotePeer())
		err = c.evaluateAndReplaceP2PRx(ctx, stream)
		if err != nil {
			c.logger.Infof(ctx, "evaluate p2p stream failed: %s", err.Error())
			return
		}
	})

	c.hostId = host.ID().String()
	c.logger.Infof(ctx, "register p2p relay success: %s", c.hostId)
	return
}

func (c *Client) dialDstRelay(ctx context.Context, dst string) (stream network.Stream, err error) {
	v, ok := c.relayMapping.Load(dst)
	if !ok {
		return nil, errors.New("destination didnt register p2p")
	}
	hostId := v.(string)
	addr := fmt.Sprintf("/p2p/%s/p2p-circuit/p2p/%s", c.relayInfo.Id, hostId)
	c.logger.Infof(ctx, "dial peer %s p2p stream: %s", dst, addr)
	relayAddr, err := multiaddr.NewMultiaddr(addr)
	if err != nil {
		return
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(relayAddr)
	if err != nil {
		return
	}

	if err = c.host.Connect(ctx, *addrInfo); err != nil {
		return
	}

	c.logger.Infof(ctx, "dial peer %s p2p stream success", dst)
	stream, err = c.host.NewStream(network.WithAllowLimitedConn(ctx, "transport"), addrInfo.ID, "/transport")
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
	v, ok := c.hub.Router().RouteString(string(dst))
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

func (c *Client) AfterDial(ctx context.Context, dst *hub.Destination) {
	if !c.cfg.P2P.Enabled {
		return
	}
	c.logger.Infof(ctx, "try connect and evaluate p2p tx: %s", dst.Ip())
	_ = c.workerPool.Submit(func() {
		stream, err := c.dialDstRelay(ctx, dst.Ip())
		if err != nil {
			c.logger.Infof(ctx, "dial p2p stream failed: %s", err.Error())
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

func (c *Client) OnFallback(ctx context.Context, dst *hub.Destination, rw protocol.ReadWriter, err error) {
}
