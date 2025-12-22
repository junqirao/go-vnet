package transport

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/logger"
	tt "go-vnet/common/tls"
	"go-vnet/device"
	"go-vnet/server/auth"
)

type (
	quicTransport struct {
		ctx     context.Context
		address string
		port    int
		conn    *quic.Conn
		streams map[string]*quic.Stream
		logger  logger.Logger
		auth    *auth.Client
		dev     *device.Config
	}
	streamWrapper struct {
		*quic.Stream
		closeCb func() error
	}
)

func NewQuicTransport(ctx context.Context, address string, port int, auth *auth.Client) (t Transport, err error) {
	qt := &quicTransport{
		address: address,
		port:    port,
		logger:  logger.DefaultLogger,
		streams: make(map[string]*quic.Stream),
		auth:    auth,
	}
	err = qt.dial(ctx)
	t = qt
	return
}

func (c *quicTransport) dial(ctx context.Context) (err error) {
	c.ctx = ctx
	// extra configs
	// tlsConfig := config.GetMappedConfig[*tls.ServerConfig](c.cfg, configKeyTLS,
	// 	// generate if not set
	// 	tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	// quicConfig := config.GetMappedConfig[*quic.ServerConfig](c.cfg, configKeyQuicConfig)

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

	c.conn = conn
	return
}

func (c *quicTransport) Connect(dev *device.Config, dst string) (wc io.ReadWriteCloser, err error) {
	c.dev = dev
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
