package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/common/protocol"
	"go-vnet/common/session"
	"go-vnet/vnet/client/hub"
)

const (
	transportTypeTcp = "tcp"
)

type (
	tcpClient struct {
		client    *Client
		transport struct {
			control   net.Conn // 控制连接
			rx        net.Conn
			conn      sync.Map // dst -> protocol.ReadWriter
			connMutex sync.Mutex
		}
		sig chan struct{}
	}
	tcpRxHook struct {
		ctx    context.Context
		c      *tcpClient
		remote string
	}
)

const (
	dispatchTypeControl = 1
	dispatchTypeIPv4    = 4
	dispatchTypeIPv6    = 6
)

func newTcpRxHook(ctx context.Context, c *tcpClient, conn net.Conn) hub.RxHook {
	return &tcpRxHook{
		ctx:    ctx,
		c:      c,
		remote: conn.RemoteAddr().String(),
	}
}

func (t *tcpRxHook) OnStart() {
	g.Log().Infof(t.ctx, "[RX] tcp accept connection: from=%v", t.remote)
}

func (t *tcpRxHook) OnClose(err error) {
	g.Log().Infof(t.ctx, "[RX] tcp close connection: from=%v, err=%v", t.remote, err)
}

func newTcpClient(client *Client) *tcpClient {
	return &tcpClient{
		client: client,
	}
}

func (c *tcpClient) Setup(ctx context.Context) (control session.SendReceiveCloser, err error) {
	c.sig = make(chan struct{})

	// 建立控制连接
	server := fmt.Sprintf("%s:%d", c.client.cfg.Address, c.client.cfg.Port)
	g.Log().Infof(ctx, "dial tcp server (control): %s", server)

	c.transport.control, err = net.DialTimeout("tcp", server, 10*time.Second)
	if err != nil {
		err = fmt.Errorf("dial control connection error: %w", err)
		return
	}

	if err = c.dispatch(ctx, c.transport.control, dispatchTypeControl, "", "", time.Second*3); err != nil {
		_ = c.transport.control.Close()
	}

	g.Log().Infof(ctx, "control connection established: remote=%v", c.transport.control.RemoteAddr())

	control = session.SendReceiverFromNetConn(c.transport.control)
	return
}

func (c *tcpClient) dispatch(ctx context.Context, conn net.Conn, typ uint8, from string, to string, timeout time.Duration) (err error) {
	buf := make([]byte, 33)
	buf[0] = typ
	length := 0
	switch typ {
	case dispatchTypeControl:
		length = 1
	case dispatchTypeIPv4:
		copy(buf[1:5], net.ParseIP(from).To4()[:4])
		copy(buf[5:9], net.ParseIP(to).To4()[:4])
		length = 9
	case dispatchTypeIPv6:
		copy(buf[1:17], net.ParseIP(from).To16()[:16])
		copy(buf[17:33], net.ParseIP(to).To16()[:16])
		length = 33
	default:
		err = fmt.Errorf("invalid dispatch type: %d", typ)
		return
	}

	g.Log().Infof(ctx, "tcp dispatch: typ=%d, data=%v, length=%d", typ, buf[:length], length)
	if _, err = conn.Write(buf); err != nil {
		return
	}

	g.Log().Infof(ctx, "waiting for dispatch ack")
	defer func() {
		if err != nil {
			g.Log().Infof(ctx, "dispatch ack error: %v", err)
		} else {
			g.Log().Infof(ctx, "dispatch ack success")
		}
	}()
	ack := make([]byte, 1)
	if err = conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return
	}
	defer func() {
		_ = conn.SetReadDeadline(time.Time{})
	}()

	if _, err = io.ReadFull(conn, ack); err != nil {
		return err
	}

	if ack[0] != 1 {
		err = fmt.Errorf("invalid ack: %d", ack[0])
	}
	return
}

func (c *tcpClient) Dial(ctx context.Context, dst string) (rw protocol.ReadWriter, err error) {
	// 检查是否已有连接
	v, ok := c.transport.conn.Load(dst)
	if ok {
		rw = v.(protocol.ReadWriter)
		return
	}

	// 建立新的数据连接
	c.transport.connMutex.Lock()
	defer c.transport.connMutex.Unlock()

	server := fmt.Sprintf("%s:%d", c.client.cfg.Address, c.client.cfg.Port)
	g.Log().Infof(ctx, "dial tcp server (data): %s, dst=%s", server, dst)

	conn, err := net.DialTimeout("tcp", server, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial data connection error: %w", err)
	}

	// dispatch
	err = c.dispatch(ctx, conn, dispatchTypeIPv4, c.client.session.IP, dst, time.Second*3)
	if err != nil {
		_ = conn.Close()
		return
	}

	transportOptions := append(c.client.transport.opts, protocol.WithType(transportTypeTcp))

	// rx
	if dst == c.client.session.IP {
		c.client.hub.HandleRx(conn, transportOptions, newTcpRxHook(ctx, c, conn))
		c.transport.rx = conn
		g.Log().Infof(ctx, "rx connection established: remote=%v", conn.RemoteAddr())
		return
	}

	rw = protocol.NewTransport(conn, transportOptions...)
	c.transport.conn.Store(dst, rw)
	g.Log().Infof(ctx, "[TX] open data connection: dst=%s, remote=%v", dst, conn.RemoteAddr())
	return
}

func (c *tcpClient) dialRx(ctx context.Context) {
	_, _ = c.Dial(ctx, c.client.session.IP)
}

func (c *tcpClient) CloseDst(ctx context.Context, dst string) {
	v, ok := c.transport.conn.LoadAndDelete(dst)
	if ok {
		if rw, ok := v.(protocol.ReadWriter); ok {
			_ = rw.Close()
			g.Log().Infof(ctx, "[TX] close data connection: dst=%s", dst)
		}
	}
}

func (c *tcpClient) OnError(ctx context.Context, e *hub.TxError) {
	g.Log().Error(ctx, e.Error())
	c.CloseDst(ctx, e.Dst().Ip())
}

func (c *tcpClient) Close() (err error) {
	select {
	case _, ok := <-c.sig:
		if ok {
			close(c.sig)
		}
		return
	default:
	}

	// 关闭控制连接
	if c.transport.control != nil {
		err = c.transport.control.Close()
		c.transport.control = nil
	}
	// 关闭接收连接
	if c.transport.rx != nil {
		err = c.transport.rx.Close()
		c.transport.rx = nil
	}

	// 关闭所有发送连接
	c.transport.conn.Range(func(key, value any) bool {
		if rw, ok := value.(protocol.ReadWriter); ok {
			_ = rw.Close()
		}
		return true
	})
	c.transport.conn.Clear()

	return
}

func (c *tcpClient) AfterHandshake(ctx context.Context, _ *session.Session) {
	g.Log().Info(ctx, "after handshake dial rx")
	c.dialRx(ctx)
}
