package client

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/auth"
	"go-vnet/common/logger"
	tt "go-vnet/common/tls"
	"go-vnet/device"
)

type (
	quicTransport struct {
		ctx     context.Context
		address string
		port    int
		conn    *quic.Conn
		streams map[string]*quic.Stream
		logger  logger.Logger
		auth    *auth.Handler
		dev     *device.Config
	}
	streamWrapper struct {
		*quic.Stream
		closeCb func() error
	}
)

func NewQuicTransport(ctx context.Context, address string, port int, auth *auth.Handler, dev *device.Config) (t Transport, err error) {
	qt := &quicTransport{
		address: address,
		port:    port,
		logger:  logger.DefaultLogger,
		streams: make(map[string]*quic.Stream),
		auth:    auth,
		dev:     dev,
	}
	err = qt.dial(ctx)
	t = qt
	return
}

func (c *quicTransport) dial(ctx context.Context) (err error) {
	c.ctx = ctx
	// extra configs
	// tlsConfig := config.GetMappedConfig[*tls.Config](c.cfg, configKeyTLS,
	// 	// generate if not set
	// 	tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	// quicConfig := config.GetMappedConfig[*quic.Config](c.cfg, configKeyQuicConfig)

	addr := fmt.Sprintf("%s:%d", c.address, c.port)
	tls := tt.GenerateTLSConfig(time.Hour*24*7, 1024)
	tls.InsecureSkipVerify = true
	conn, err := quic.DialAddr(context.Background(), addr, tls, &quic.Config{
		KeepAlivePeriod: time.Second * 3,
		EnableDatagrams: true,
	})
	if err != nil {
		return
	}

	bs, err := c.auth.AuthorizedHandler().Make(ctx, map[string]any{
		"address": c.dev.CIDR,
	})
	if err != nil {
		return
	}

	if err = conn.SendDatagram(bs); err != nil {
		return
	}
	c.conn = conn
	return
}

func (c *quicTransport) Connect(dst string) (wc io.ReadWriteCloser, err error) {
	stream, err := c.conn.OpenStreamSync(context.Background())
	if err != nil {
		c.logger.Errorf(c.ctx, "open stream error: %s", err.Error())
		// retry dial
		if err = c.dial(c.ctx); err != nil {
			return
		}
		stream, err = c.conn.OpenStreamSync(context.Background())
		if err != nil {
			return
		}
	}
	wc = &streamWrapper{
		Stream: stream,
		closeCb: func() error {
			delete(c.streams, dst)
			return stream.Close()
		},
	}
	c.streams[dst] = stream
	return
}

func (s *streamWrapper) Close() error {
	return s.closeCb()
}
