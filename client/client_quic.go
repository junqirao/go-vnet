package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
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
		stream      atomic.Pointer[quic.Stream] // 优化：使用atomic指针，无锁访问stream
		bytesSent   atomic.Uint64               // 优化：使用atomic替换锁，更快的计数器
		lastChecked atomic.Int64                // 优化：使用atomic存储时间戳
	}
	quicSendReceiver struct {
		*quic.Conn
	}
)

var (
	defaultQuicConfig = &quic.Config{
		KeepAlivePeriod:                time.Second * 3,
		EnableDatagrams:                true,
		MaxIdleTimeout:                 time.Second * 30,
		MaxIncomingStreams:             1000,
		MaxIncomingUniStreams:          1000,
		MaxStreamReceiveWindow:         8 * 1024 * 1024,  // 优化：增大流接收窗口到8MB，提高吞吐量
		InitialConnectionReceiveWindow: 16 * 1024 * 1024, // 优化：增大初始连接接收窗口到16MB，加快冷启动
		MaxConnectionReceiveWindow:     64 * 1024 * 1024, // 优化：设置连接接收窗口上限64MB
		Allow0RTT:                      true,
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

	// 优化：设置默认QUIC配置以减少传输延迟
	quicConfig := config.GetMappedConfig[*quic.Config](c.cfg, ConfigKeyQuicConfig, defaultQuicConfig)

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
			now := time.Now().UnixNano()
			c.session.streams.Range(func(key, value any) bool {
				wrapper := value.(*streamWrapper)
				// 优化：使用atomic.Load读取stream指针
				stream := wrapper.stream.Load()
				lastChecked := wrapper.lastChecked.Load()
				bytesSent := wrapper.bytesSent.Load()

				// 检查是否空闲
				if bytesSent == 0 && now-lastChecked > int64(time.Second*30) {
					if wrapper.stream.CompareAndSwap(stream, nil) {
						_ = stream.Close()
						c.session.streams.Delete(key)
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
		wrapper = &streamWrapper{}
		wrapper.stream.Store(stream)
		wrapper.lastChecked.Store(time.Now().UnixNano())
		c.session.streams.Store(dst, wrapper)
		c.logger.Infof(c.ctx, "create tx stream: %s", dst)
	} else {
		wrapper = v.(*streamWrapper)
	}

	// 优化：使用atomic更新计数器，无锁操作
	wrapper.bytesSent.Add(uint64(n))

	// 优化：使用atomic.Load获取stream，无锁操作
	stream := wrapper.stream.Load()
	if stream == nil {
		stream, err = c.session.conn.OpenStream()
		if err != nil {
			return err
		}
		// 优化：使用atomic.Store设置stream
		wrapper.stream.Store(stream)
	}

	_, err = stream.Write(buf[:n])
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

// writeDevice - 优化版本：简化写入逻辑，TUN设备通常不会部分写入
func (c *QuicClient) writeDevice(src io.Reader) (written int64, err error) {
	var (
		buf = make([]byte, c.session.DispatchedDevice.MTU*100)
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
			wn, err := c.dev.Write(buf[0:nr])
			if err != nil {
				return written, err
			}
			if wn > 0 {
				written += int64(wn)
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
