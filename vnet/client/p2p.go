package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/network"

	"go-vnet/common/protocol"
	"go-vnet/vnet/client/hub"
)

const (
	acceptableLatency = 1000
)

var (
	evaluateFail = [1]byte{1}
	evaluatePass = [1]byte{2}
)

func (c *Client) evaluateP2PTx(ctx context.Context, dst *hub.Destination, stream network.Stream) (err error) {
	// ping and wait result
	start := time.Now()
	_ = stream.SetWriteDeadline(start.Add(time.Second * 2))
	_ = stream.SetReadDeadline(start.Add(time.Second * 2))
	defer func() {
		_ = stream.SetWriteDeadline(time.Time{})
		_ = stream.SetReadDeadline(time.Time{})
	}()
	if _, err = stream.Write([]byte(dst.Ip())); err != nil {
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
	dst.ReplaceWithFallback(protocol.NewTransport(stream), func(rw protocol.ReadWriter, err error) {
		_ = rw.Close()
		c.logger.Infof(ctx, "dst %s connection fallback to proxy: %s", dst.Ip(), err.Error())
	})
	c.logger.Infof(ctx, "replace dst %s to p2p connection", dst.Ip())
	return
}

func (c *Client) evaluateP2PRx(ctx context.Context, stream network.Stream) (err error) {
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
		ipBuf = make([]byte, 16)
	)

	dst := ipBuf[:n]
	if n, err = stream.Read(ipBuf); err != nil {
		return
	}
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
	return
}
