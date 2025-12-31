package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
)

type (
	QuicClient struct {
		*Client
		// quic
		tlsConfig  *tls.Config
		quicConfig *quic.Config
		session    *QuicSession
	}
	QuicSession struct {
		*Session
		conn    *quic.Conn
		streams sync.Map // dst : *streamWrapper
	}
	streamWrapper struct {
		stream      *quic.Stream
		bytesSent   uint64
		lastChecked time.Time
		mu          sync.Mutex
	}
	quicSendReceiver struct {
		*quic.Conn
	}
)

func (q *QuicSession) Close() error {
	if q.err == nil {
		q.err = errors.New("close manually")
	}
	return q.conn.CloseWithError(1, q.err.Error())
}

func SendReceiverFromQuicConn(conn *quic.Conn) SendReceiver {
	return &quicSendReceiver{conn}
}

func (q quicSendReceiver) Send(data []byte) (err error) {
	return q.SendDatagram(data)
}

func (q quicSendReceiver) Receive(ctx context.Context) (data []byte, err error) {
	return q.ReceiveDatagram(ctx)
}

func NewQuicClient(c *Client) *QuicClient {
	// extra configs
	tlsConfig := config.GetMappedConfig[*tls.Config](c.cfg, ConfigKeyTLS,
		// generate if not set
		tt.GenerateTLSConfig(time.Hour*24*7, 1024))
	quicConfig := config.GetMappedConfig[*quic.Config](c.cfg, ConfigKeyQuicConfig, &quic.Config{
		KeepAlivePeriod: time.Second * 3,
		EnableDatagrams: true,
	})
	if c.cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	qc := &QuicClient{
		tlsConfig:  tlsConfig,
		quicConfig: quicConfig,
	}

	qc.Client = c

	return qc
}

func (c *QuicClient) Dial(ctx context.Context) (session *Session, err error) {
	c.ctx = ctx
	addr := fmt.Sprintf("%s:%d", c.cfg.Address, c.cfg.Port)
	conn, err := quic.DialAddr(ctx, addr, c.tlsConfig, c.quicConfig)
	if err != nil {
		return
	}

	// get payload and overwrite network id
	payload := c.cfg.authPayload
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["network_id"] = c.cfg.NetworkId

	// do auth
	resp, err := c.auth.Auth(ctx, payload,
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

	joined := JoinNetworkResponse{}
	bs, _ := json.Marshal(resp)
	_ = json.Unmarshal(bs, &joined)

	c.session = &QuicSession{
		conn: conn,
	}
	session = &Session{
		SendReceiver:     SendReceiverFromQuicConn(conn),
		DispatchedDevice: joined.Device,
		Closer:           c.session,
	}
	c.session.Session = session

	c.manager = NewManager(session)
	go c.cleanupIdleStreams()
	go c.acceptStreamLoop()
	return
}

func (c *QuicClient) cleanupIdleStreams() {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-c.sig:
			return
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			c.session.streams.Range(func(key, value any) bool {
				wrapper := value.(*streamWrapper)
				wrapper.mu.Lock()
				if wrapper.stream == nil {
					wrapper.mu.Unlock()
					return true
				}
				if wrapper.bytesSent == 0 && time.Since(wrapper.lastChecked) > time.Second*30 {
					_ = wrapper.stream.Close()
					wrapper.stream = nil
					c.session.streams.Delete(key)
					c.logger.Infof(c.ctx, "close idle stream: %s", key)
				}
				wrapper.bytesSent = 0
				wrapper.lastChecked = now
				wrapper.mu.Unlock()
				return true
			})
		}
	}
}

func (c *QuicClient) SendToServer(dst string, buf []byte, n int) (err error) {
	// reuse stream
	// close stream if error or no transport for 30s
	var wrapper *streamWrapper
	v, ok := c.session.streams.Load(dst)
	if !ok {
		stream, err := c.session.conn.OpenStream()
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
		wrapper = &streamWrapper{
			stream:      stream,
			lastChecked: time.Now(),
		}
		c.session.streams.Store(dst, wrapper)
		c.logger.Infof(c.ctx, "create tx stream: %s", dst)
	} else {
		wrapper = v.(*streamWrapper)
	}

	wrapper.mu.Lock()
	defer wrapper.mu.Unlock()
	if wrapper.stream == nil {
		wrapper.stream, err = c.session.conn.OpenStream()
		if err != nil {
			return err
		}
	}
	wrapper.bytesSent += uint64(n)
	_, err = wrapper.stream.Write(buf[:n])
	return err
}

func (c *QuicClient) acceptStreamLoop() {
	for {
		select {
		case <-c.sig:
			return
		case <-c.ctx.Done():
			return
		default:
		}
		stream, err := c.session.conn.AcceptStream(context.Background())
		if err != nil {
			return
		}
		go c.handleStream(stream)
	}
}

func (c *QuicClient) handleStream(stream *quic.Stream) {
	c.logger.Infof(c.ctx, "handle rx stream: %v", stream.StreamID())
	defer func() {
		_ = stream.Close()
	}()

	written, err := c.writeDevice(stream)
	c.logger.Infof(c.ctx, "handle rx stream stopped: %d bytes written, err=%v", written, err)
}

func (c *QuicClient) writeDevice(src io.Reader) (written int64, err error) {
	var (
		buf = make([]byte, c.session.DispatchedDevice.MTU)
		nr  int
		er  error
	)

	for {
		select {
		case <-c.ctx.Done():
			return written, c.ctx.Err()
		case <-c.sig:
			return
		default:
		}
		nr, er = src.Read(buf)
		if nr > 0 {
			// Write all bytes to device, handling partial writes
			var wn int
			wn, err = c.dev.Write(buf[0:nr])
			if err != nil {
				// If write failed partially, retry with remaining bytes
				if wn > 0 && wn < nr {
					written += int64(wn)
					remaining := buf[wn:nr]
					for len(remaining) > 0 {
						var w int
						w, err = c.dev.Write(remaining)
						if err != nil {
							break
						}
						if w <= 0 {
							err = io.ErrShortWrite
							break
						}
						written += int64(w)
						remaining = remaining[w:]
					}
				} else if wn < 0 {
					wn = 0
					if err == nil {
						err = errors.New("invalid write")
					}
					written += int64(wn)
				} else {
					written += int64(wn)
				}
			} else {
				written += int64(wn)
				// Handle partial write even if no error returned
				if wn < nr {
					remaining := buf[wn:nr]
					for len(remaining) > 0 {
						var w int
						w, err = c.dev.Write(remaining)
						if err != nil {
							break
						}
						if w <= 0 {
							err = io.ErrShortWrite
							break
						}
						written += int64(w)
						remaining = remaining[w:]
					}
				}
			}
			if err != nil {
				break
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}
	return written, err
}
