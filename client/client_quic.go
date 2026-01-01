package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	"go-vnet/common/session"
	tt "go-vnet/common/tls"
)

type (
	quicClient struct {
		*Client
		// quic
		tlsConfig  *tls.Config
		quicConfig *quic.Config
		streams    sync.Map
	}
	quicStreamWrapper struct {
		stream      atomic.Pointer[quic.Stream]
		bytesSent   atomic.Uint64
		lastChecked atomic.Int64
	}
)

func newQuicClient(c *Client) *quicClient {
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](c.cfg, ConfigKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))

	quicConfig := config.GetMappedConfig[*quic.Config](c.cfg, ConfigKeyQuicConfig, config.DefaultQuicConfig)

	if c.cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	qc := &quicClient{
		tlsConfig:  tlsConfig,
		quicConfig: quicConfig,
	}

	qc.Client = c
	return qc
}

func (c *quicClient) Dial(ctx context.Context) (sr session.SendReceiveCloser, conn any, err error) {
	c.ctx = ctx
	addr := fmt.Sprintf("%s:%d", c.cfg.Address, c.cfg.Port)
	cc, err := quic.DialAddr(ctx, addr, c.tlsConfig, c.quicConfig)
	if err != nil {
		return
	}

	conn = cc
	sr = session.SendReceiverFromQuicConn(cc)
	go c.cleanupIdleStreams()
	return
}

func (c *quicClient) cleanupIdleStreams() {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-c.sig:
			return
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now().UnixNano()
			c.streams.Range(func(key, value any) bool {
				wrapper := value.(*quicStreamWrapper)
				stream := wrapper.stream.Load()
				lastChecked := wrapper.lastChecked.Load()
				bytesSent := wrapper.bytesSent.Load()

				// check if stream is idle
				if bytesSent == 0 && now-lastChecked > int64(time.Second*30) {
					if wrapper.stream.CompareAndSwap(stream, nil) {
						_ = stream.Close()
						c.streams.Delete(key)
						c.logger.Infof(c.ctx, "close idle stream: %s", key)
					}
				} else {
					wrapper.bytesSent.Store(0)
					wrapper.lastChecked.Store(now)
				}
				return true
			})
		}
	}
}

func (c *quicClient) SendToServer(dst string, buf []byte, n int) (err error) {
	// reuse stream
	// close stream if error or no transport for 30s
	var wrapper *quicStreamWrapper
	v, ok := c.streams.Load(dst)
	if !ok {
		conn := c.session.Conn.(*quic.Conn)
		stream, err := conn.OpenStream()
		if err != nil {
			return err
		}
		// write dst and wait for ack
		if _, err = stream.Write([]byte(dst)); err != nil {
			return err
		}
		// wait for server ack (byte 1)
		ack := make([]byte, 1)
		_, err = stream.Read(ack)
		if err != nil || ack[0] != 1 {
			if err == nil {
				err = fmt.Errorf("invalid ack: %d", ack[0])
			}
			return err
		}
		wrapper = &quicStreamWrapper{}
		wrapper.stream.Store(stream)
		wrapper.lastChecked.Store(time.Now().UnixNano())
		c.streams.Store(dst, wrapper)
		c.logger.Infof(c.ctx, "create tx stream: %s", dst)
	} else {
		wrapper = v.(*quicStreamWrapper)
	}

	// update counter
	wrapper.bytesSent.Add(uint64(n))
	stream := wrapper.stream.Load()
	if stream == nil {
		conn := c.session.Conn.(*quic.Conn)
		stream, err = conn.OpenStream()
		if err != nil {
			return err
		}
		wrapper.stream.Store(stream)
	}

	_, err = stream.Write(buf[:n])
	return err
}

func (c *quicClient) ReadFromServerAndWriteToDevice() {
	conn := c.session.Conn.(*quic.Conn)
	for {
		select {
		case <-c.sig:
			return
		case <-c.ctx.Done():
			return
		default:
		}
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			return
		}
		go func() {
			_, _ = c.writeDevice(stream)
		}()
	}
}
