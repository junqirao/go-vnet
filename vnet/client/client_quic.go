package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	"go-vnet/common/protocol"
	"go-vnet/common/session"
	tt "go-vnet/common/tls"
	"go-vnet/vnet/client/hub"
)

type (
	quicClient struct {
		client    *Client
		transport struct {
			conn    *quic.Conn
			streams sync.Map
		}
		sig chan struct{}
	}
	quicRxHook struct {
		ctx      context.Context
		c        *quicClient
		remote   string
		streamId quic.StreamID
	}
)

func newQuicRxHook(ctx context.Context, c *quicClient, stream *quic.Stream) hub.RxHook {
	return &quicRxHook{
		ctx:      ctx,
		c:        c,
		remote:   c.transport.conn.RemoteAddr().String(),
		streamId: stream.StreamID(),
	}
}

func (q *quicRxHook) OnStart() {
	q.c.client.logger.Infof(q.ctx, "[RX] quic accept stream: id=%v,from=%v",
		q.streamId, q.remote)
}

func (q *quicRxHook) OnClose(err error) {
	q.c.client.logger.Infof(q.ctx, "[RX] quic close stream: id=%v,from=%v,err=%v",
		q.streamId, q.remote, err)
}

func newQuicClient(client *Client) *quicClient {
	return &quicClient{
		client: client,
	}
}

func (c *quicClient) Setup(ctx context.Context) (control session.SendReceiveCloser, err error) {
	c.sig = make(chan struct{})
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](c.client.cfg, ConfigKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))

	quicConfig := config.GetMappedConfig[*quic.Config](c.client.cfg, ConfigKeyQuicConfig, config.DefaultQuicConfig)

	if c.client.cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}
	server := fmt.Sprintf("%s:%d", c.client.cfg.Address, c.client.cfg.Port)
	c.client.logger.Infof(ctx, "dial quic server: %s", server)
	c.transport.conn, err = quic.DialAddr(ctx,
		server, tlsConfig, quicConfig)
	if err != nil {
		return
	}

	control = session.SendReceiverFromQuicConn(c.transport.conn)
	go c.rxLoop(ctx)
	return
}

func (c *quicClient) rxLoop(ctx context.Context) {
	var cancels []func()
	defer func() {
		for _, cancel := range cancels {
			cancel()
		}
	}()

	for {
		select {
		case <-c.sig:
			return
		default:
		}
		stream, err := c.transport.conn.AcceptStream(ctx)
		if err != nil {
			c.client.logger.Errorf(ctx, "accept stream error: %v", err.Error())
			return
		}
		hook := newQuicRxHook(ctx, c, stream)
		cancels = append(cancels, c.client.hub.HandleRx(stream, hook))
	}
}

func (c *quicClient) Dial(ctx context.Context, dst string) (rw protocol.ReadWriter, err error) {
	v, ok := c.transport.streams.Load(dst)
	if ok {
		rw = v.(protocol.ReadWriter)
		return
	}
	stream, err := c.transport.conn.OpenStreamSync(ctx)
	if err != nil {
		return
	}
	rw = protocol.NewTransport(stream)
	c.transport.streams.Store(dst, rw)
	c.client.logger.Infof(ctx, "[TX] open stream: id=%v,dst=%s", stream.StreamID(), dst)
	return
}

func (c *quicClient) CloseDst(ctx context.Context, dst string) {
	v, ok := c.transport.streams.LoadAndDelete(dst)
	if ok {
		if rw, ok := v.(protocol.ReadWriter); ok {
			if stream, ok := rw.Upstream().(*quic.Stream); ok {
				id := stream.StreamID()
				_ = stream.Close()
				c.client.logger.Infof(ctx, "[TX] close stream: id=%v,dst=%s", id, dst)
			}
		}
	}
}

func (c *quicClient) OnError(ctx context.Context, e *hub.TxError) {
	c.client.logger.Error(ctx, e.Error())
	c.CloseDst(ctx, e.Dst().Ip())
}

func (c *quicClient) Close() (err error) {
	select {
	case _, ok := <-c.sig:
		if ok {
			close(c.sig)
		}
		return
	default:

	}
	if c.transport.conn != nil {
		err = c.transport.conn.CloseWithError(quic.ApplicationErrorCode(0), "exit")
		c.transport.conn = nil
	}
	c.transport.streams.Range(func(key, value any) bool {
		if stream, ok := value.(protocol.ReadWriter); ok {
			if quicStream, ok := stream.Upstream().(*quic.Stream); ok {
				_ = quicStream.Close()
			}
		}
		return true
	})
	c.transport.streams.Clear()
	return
}

func (c *quicClient) AfterHandshake(ctx context.Context, session *session.Session) {
	// do nothing
	return
}
