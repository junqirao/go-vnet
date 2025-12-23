package transport

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	tt "go-vnet/common/tls"
)

type (
	quicTransport struct {
		ctx     context.Context
		cfg     *Config
		conn    *quic.Conn
		streams map[string]*quic.Stream
		logger  logger.Logger
		auth    *auth.Client
		joined  JoinNetworkResponse
	}
	streamWrapper struct {
		*quic.Stream
		closeCb func() error
	}
)

func NewQuicTransport(ctx context.Context, cfg *Config, auth *auth.Client) (t Transport, err error) {
	qt := &quicTransport{
		logger:  logger.DefaultLogger,
		streams: make(map[string]*quic.Stream),
		auth:    auth,
		cfg:     cfg,
	}
	err = qt.dial(ctx)
	t = qt
	return
}

func (c *quicTransport) dial(ctx context.Context) (err error) {
	c.ctx = ctx
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](c.cfg, configKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	quicConfig := config.GetMappedConfig[*quic.Config](c.cfg, configKeyQuicConfig, &quic.Config{
		KeepAlivePeriod: time.Second * 3,
		EnableDatagrams: true,
	})
	if c.cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	addr := fmt.Sprintf("%s:%d", c.cfg.Address, c.cfg.Port)
	conn, err := quic.DialAddr(context.Background(), addr, tlsConfig, quicConfig)
	if err != nil {
		return
	}

	resp, err := c.auth.Auth(ctx, c.cfg.authPayload,
		func(ctx context.Context, in []byte) (out []byte, err error) {
			if err = conn.SendDatagram(in); err != nil {
				return
			}
			return conn.ReceiveDatagram(ctx)
		},
	)
	if err != nil {
		return
	}

	c.joined = JoinNetworkResponse{}
	bs, _ := json.Marshal(resp)
	_ = json.Unmarshal(bs, &c.joined)

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

func (c *quicTransport) JoinedNetwork() JoinNetworkResponse {
	return c.joined
}

func (s *streamWrapper) Close() error {
	return s.closeCb()
}
