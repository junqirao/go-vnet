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

type quicClient struct {
	client    *Client
	transport struct {
		conn    *quic.Conn
		streams sync.Map
	}
}

func newQuicClient(client *Client) *quicClient {
	return &quicClient{
		client: client,
	}
}

func (c *quicClient) Run(ctx context.Context) (err error) {
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

	go func() {
		for {
			stream, err := c.transport.conn.AcceptStream(ctx)
			if err != nil {
				c.client.logger.Errorf(ctx, "accept stream error: %v", err.Error())
				return
			}
			c.client.logger.Infof(ctx, "accept stream: %v", stream.StreamID())
			go c.client.hub.HandleRx(stream, c.transport.conn.RemoteAddr().String())
		}
	}()
	return
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
	return
}

func (c *quicClient) CloseDst(ctx context.Context, dst string) {
	v, ok := c.transport.streams.LoadAndDelete(dst)
	if ok {
		if rw, ok := v.(protocol.ReadWriter); ok {
			if stream, ok := rw.Upstream().(*quic.Stream); ok {
				id := stream.StreamID()
				_ = stream.Close()
				c.client.logger.Infof(ctx, "close stream: %v->%s", id, dst)
			}
		}
	}
}

func (c *quicClient) OnError(ctx context.Context, e *hub.TxError) {
	c.client.logger.Error(ctx, e.Error())
	c.CloseDst(ctx, e.Dst().Ip())
}

func (c *quicClient) Handshake(ctx context.Context, payload map[string]any) (sess *session.Session, sr session.SendReceiveCloser, err error) {
	resp := &handshakeResponse{}
	err = c.client.auth.AuthPtr(ctx, payload,
		func(ctx context.Context, in []byte) (out []byte, err error) {
			if err = c.transport.conn.SendDatagram(in); err != nil {
				return
			}
			return c.transport.conn.ReceiveDatagram(ctx)
		},
		resp,
	)
	sess = resp.Session
	sr = session.SendReceiverFromQuicConn(c.transport.conn)
	c.client.logger.Infof(ctx, "handshake success: id=%v,ip=%v", sess.SessionId, sess.IP)
	return
}
