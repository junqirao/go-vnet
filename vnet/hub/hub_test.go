package hub

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"testing"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/vnet/protocol"
)

type (
	testDestination struct {
		buf *threadSafeBuffer
	}
)

func newTestDestination() *testDestination {
	return &testDestination{
		buf: newThreadSafeBuffer(),
	}
}

func (t *testDestination) OnDialRx(ctx context.Context) (rw protocol.ReadWriter, err error) {
	return protocol.NewTransport(t.buf), nil
}

func (t *testDestination) OnError(e *TxError) {
	panic(e)
}

// StartPrinting 启动一个协程，持续从 buf 读取数据并打印
func (t *testDestination) StartPrinting() {
	go func() {
		buf := make([]byte, 65535)
		for {
			n, err := t.buf.Read(buf)
			if err != nil {
				return
			}
			fmt.Printf("[Received][%d] %v\n", n, buf[:n])
		}
	}()
}

// threadSafeBuffer 线程安全的 buffer 实现
type threadSafeBuffer struct {
	mu     sync.RWMutex
	buffer []byte
	cond   *sync.Cond
}

func newThreadSafeBuffer() *threadSafeBuffer {
	t := &threadSafeBuffer{
		buffer: make([]byte, 0, 65535),
	}
	t.cond = sync.NewCond(&t.mu)
	return t
}

func (t *threadSafeBuffer) Read(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for len(t.buffer) == 0 {
		t.cond.Wait()
	}

	n = copy(p, t.buffer)
	t.buffer = t.buffer[n:]
	return n, nil
}

func (t *threadSafeBuffer) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buffer = append(t.buffer, p...)
	t.cond.Broadcast()
	return len(p), nil
}

func TestHub(t *testing.T) {
	var (
		err error
		ip  = "192.168.99.2"
		mtu = 1392
	)

	pfx, _ := netip.ParsePrefix(fmt.Sprintf("%s/24", ip))

	dev, err := tun.New(tun.Options{
		Name:         "tun0",
		Inet4Address: []netip.Prefix{pfx},
		Inet6Address: nil,
		MTU:          uint32(mtu),
	})
	if err != nil {
		panic(err)
		return
	}
	h := NewHub(Config{
		Name:          "test",
		MTU:           mtu,
		MaxRxEventBuf: 1024,
		MaxTxEventBuf: 1024,
	}, dev)

	dst := newTestDestination()
	h.router.Register("192.168.99.3/32", NewDestination(context.Background(), dst, h))
	dst.StartPrinting()
	h.Start()
}
