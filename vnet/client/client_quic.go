package client

import (
	"context"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
	"go-vnet/vnet/hub"
	"go-vnet/vnet/protocol"
)

type quicClient struct {
	client    *Client
	transport struct {
		conn   *quic.Conn
		stream *quic.Stream
	}
}

func newQuicClient(client *Client) *quicClient {
	return &quicClient{
		client: client,
	}
}

func (c *quicClient) Run(ctx context.Context) (err error) {
	tls := tt.GenerateTLSConfig(time.Hour*24*7, 1024)
	tls.InsecureSkipVerify = true
	c.transport.conn, err = quic.DialAddr(ctx,
		c.client.cfg.Server, tls, config.DefaultQuicConfig)
	if err != nil {
		return
	}

	go func() {
		for {
			stream, err := c.transport.conn.AcceptStream(ctx)
			if err != nil {
				c.client.logger.Errorf(ctx, "accept stream error: %v", err.Error())
				return
			}
			c.client.hub.HandleRx(stream)
		}
	}()
	return
}

func (c *quicClient) OnDialRx(ctx context.Context) (rw protocol.ReadWriter, err error) {
	if c.transport.stream != nil {
		return protocol.NewTransport(c.transport.stream), nil
	}
	stream, err := c.transport.conn.AcceptStream(ctx)
	if err != nil {
		return
	}
	c.transport.stream = stream
	return protocol.NewTransport(stream), nil
}

func (c *quicClient) OnError(ctx context.Context, e *hub.TxError) {
	c.client.logger.Error(ctx, e.Error())
	if c.transport.stream != nil {
		_ = c.transport.stream.Close()
		c.transport.stream = nil
	}
}
