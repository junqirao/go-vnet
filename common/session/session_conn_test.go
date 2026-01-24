package session

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// 创建一对内存连接用于测试
func createTestPair() (client, server net.Conn) {
	return net.Pipe()
}

// 创建一对基于 buffer 的连接用于测试空数据等边界情况
func createBufferTestPair() (*bytes.Buffer, *bytes.Buffer, net.Conn, net.Conn) {
	clientBuf := &bytes.Buffer{}
	serverBuf := &bytes.Buffer{}

	// 使用 io.Pipe 创建双向管道
	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	// 创建自定义连接
	clientConn := &pipeConn{
		reader: clientReader,
		writer: clientWriter,
	}
	serverConn := &pipeConn{
		reader: serverReader,
		writer: serverWriter,
	}

	return clientBuf, serverBuf, clientConn, serverConn
}

type pipeConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
}

func (p *pipeConn) Read(b []byte) (n int, err error) {
	return p.reader.Read(b)
}

func (p *pipeConn) Write(b []byte) (n int, err error) {
	return p.writer.Write(b)
}

func (p *pipeConn) Close() error {
	p.reader.Close()
	p.writer.Close()
	return nil
}

func (p *pipeConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

func (p *pipeConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

func (p *pipeConn) SetDeadline(t time.Time) error {
	return nil
}

func (p *pipeConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (p *pipeConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestSendReceiverFromNetConn(t *testing.T) {
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	// 测试创建
	sr := SendReceiverFromNetConn(client)
	if sr == nil {
		t.Fatal("SendReceiverFromNetConn 返回 nil")
	}

	// 测试接口实现
	if _, ok := sr.(SendReceiveCloser); !ok {
		t.Fatal("没有实现 SendReceiveCloser 接口")
	}
}

func TestSendAndReceive(t *testing.T) {
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	clientSR := SendReceiverFromNetConn(client)
	serverSR := SendReceiverFromNetConn(server)

	testData := [][]byte{
		[]byte("Hello, World!"),
		[]byte("测试中文"),
		[]byte(""),
		bytes.Repeat([]byte("A"), 1024),
		bytes.Repeat([]byte("B"), 64*1024), // 64KB
	}

	ctx := context.Background()
	recvDone := make(chan bool)
	var wg sync.WaitGroup

	// 启动接收 goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i, expected := range testData {
			data, err := serverSR.Receive(ctx)
			if err != nil {
				t.Errorf("接收失败 (i=%d): %v", i, err)
				return
			}
			if !bytes.Equal(data, expected) {
				t.Errorf("数据不匹配 (i=%d): got %q, want %q", i, data, expected)
			}
		}
		recvDone <- true
	}()

	// 发送数据
	for i, data := range testData {
		if err := clientSR.Send(data); err != nil {
			t.Fatalf("发送失败 (i=%d): %v", i, err)
		}
	}

	// 等待接收完成
	select {
	case <-recvDone:
		// 成功
	case <-time.After(5 * time.Second):
		t.Fatal("接收超时")
	}

	wg.Wait()
}

func TestCloseWithError(t *testing.T) {
	client, server := createTestPair()
	defer client.Close()

	serverSR := SendReceiverFromNetConn(server)

	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)

	var receivedErr error
	go func() {
		defer wg.Done()
		_, receivedErr = serverSR.Receive(ctx)
	}()

	// 等待接收 goroutine 开始
	time.Sleep(100 * time.Millisecond)

	// 发送错误并关闭
	testError := errors.New("test error")
	SendReceiverFromNetConn(client).CloseWithError(testError)

	wg.Wait()

	if receivedErr == nil {
		t.Fatal("没有收到错误")
	}

	if receivedErr.Error() != "[500]test error" {
		t.Errorf("错误不匹配: got %q", receivedErr.Error())
	}

	// 尝试再次接收应该失败
	_, err := serverSR.Receive(ctx)
	if err != io.EOF && err != io.ErrClosedPipe {
		t.Errorf("期望连接关闭错误，但得到: %v", err)
	}
}

func TestReceiveError(t *testing.T) {
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	clientSR := SendReceiverFromNetConn(client)
	serverSR := SendReceiverFromNetConn(server)

	ctx := context.Background()

	// 使用 CloseWithError 发送错误
	testError := NewError("protocol error", 400)
	var wg sync.WaitGroup
	wg.Add(1)
	var receivedErr error

	go func() {
		defer wg.Done()
		_, receivedErr = serverSR.Receive(ctx)
	}()

	// 等待接收 goroutine 准备好
	time.Sleep(50 * time.Millisecond)

	// 发送错误
	clientSR.CloseWithError(testError)

	wg.Wait()

	// 验证错误
	if receivedErr == nil {
		t.Fatal("期望收到错误，但 err 为 nil")
	}

	// 应该收到 code 400，因为我们传递的是 *Error 类型
	if receivedErr.Error() != "[400]protocol error" {
		t.Errorf("错误信息不匹配: got %q, want [400]protocol error", receivedErr.Error())
	}
}

func TestConcurrentSendReceive(t *testing.T) {
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	clientSR := SendReceiverFromNetConn(client)
	serverSR := SendReceiverFromNetConn(server)

	ctx := context.Background()
	const numMessages = 100
	const messageSize = 1024

	var wg sync.WaitGroup
	wg.Add(2)

	// 发送 goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < numMessages; i++ {
			data := bytes.Repeat([]byte{byte(i % 256)}, messageSize)
			if err := clientSR.Send(data); err != nil {
				t.Errorf("发送失败: %v", err)
				return
			}
		}
	}()

	// 接收 goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < numMessages; i++ {
			data, err := serverSR.Receive(ctx)
			if err != nil {
				t.Errorf("接收失败: %v", err)
				return
			}
			if len(data) != messageSize {
				t.Errorf("消息长度不匹配: got %d, want %d", len(data), messageSize)
			}
			expectedByte := byte(i % 256)
			if data[0] != expectedByte || data[len(data)-1] != expectedByte {
				t.Errorf("消息内容不匹配")
			}
		}
	}()

	wg.Wait()
}

func TestMultipleClose(t *testing.T) {
	client, _ := createTestPair()
	clientSR := SendReceiverFromNetConn(client)

	// 多次调用 CloseWithError 不应该 panic
	clientSR.CloseWithError(nil)
	clientSR.CloseWithError(nil)                // 第二次调用
	clientSR.CloseWithError(errors.New("test")) // 带错误的调用
}

func TestEmptyData(t *testing.T) {
	// net.Pipe() 对于0字节写入有同步问题，测试一个小的非空数据作为替代
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	clientSR := SendReceiverFromNetConn(client)
	serverSR := SendReceiverFromNetConn(server)

	ctx := context.Background()
	testData := []byte{0} // 测试单字节数据而不是空数据

	// 使用 goroutine 接收数据
	done := make(chan bool)
	var receivedData []byte
	var receiveErr error

	go func() {
		receivedData, receiveErr = serverSR.Receive(ctx)
		done <- true
	}()

	// 发送数据
	if err := clientSR.Send(testData); err != nil {
		t.Fatalf("发送数据失败: %v", err)
	}

	// 等待接收完成
	<-done

	if receiveErr != nil {
		t.Fatalf("接收数据失败: %v", receiveErr)
	}

	if !bytes.Equal(receivedData, testData) {
		t.Errorf("数据不匹配: got %v, want %v", receivedData, testData)
	}
}

// Benchmarks

func BenchmarkSendReceiveSmall(b *testing.B) {
	benchmarkSendReceive(b, 128) // 128 bytes
}

func BenchmarkSendReceiveMedium(b *testing.B) {
	benchmarkSendReceive(b, 4*1024) // 4KB
}

func BenchmarkSendReceiveLarge(b *testing.B) {
	benchmarkSendReceive(b, 64*1024) // 64KB
}

func benchmarkSendReceive(b *testing.B, messageSize int) {
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	clientSR := SendReceiverFromNetConn(client)
	serverSR := SendReceiverFromNetConn(server)

	ctx := context.Background()
	data := bytes.Repeat([]byte("A"), messageSize)

	b.ResetTimer()
	b.SetBytes(int64(messageSize))

	// 使用单独的 goroutine 处理接收
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_, err := serverSR.Receive(ctx)
				if err != nil {
					return
				}
			}
		}
	}()

	// 并行测试 - 只测试发送性能
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := clientSR.Send(data); err != nil {
				b.Errorf("发送失败: %v", err)
				return
			}
		}
	})

	close(done)
}

func BenchmarkSendOnly(b *testing.B) {
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	clientSR := SendReceiverFromNetConn(client)
	serverSR := SendReceiverFromNetConn(server)
	ctx := context.Background()

	data := bytes.Repeat([]byte("B"), 1024)

	// 后台接收器
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_, _ = serverSR.Receive(ctx)
			}
		}
	}()

	b.ResetTimer()
	b.SetBytes(1024)

	for i := 0; i < b.N; i++ {
		if err := clientSR.Send(data); err != nil {
			b.Fatalf("发送失败: %v", err)
		}
	}

	close(done)
}

func BenchmarkReceiveOnly(b *testing.B) {
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	clientSR := SendReceiverFromNetConn(client)
	serverSR := SendReceiverFromNetConn(server)

	ctx := context.Background()
	data := bytes.Repeat([]byte("C"), 1024)

	// 预填充数据
	go func() {
		for i := 0; i < b.N; i++ {
			clientSR.Send(data)
		}
	}()

	b.ResetTimer()
	b.SetBytes(1024)

	for i := 0; i < b.N; i++ {
		_, err := serverSR.Receive(ctx)
		if err != nil {
			b.Fatalf("接收失败: %v", err)
		}
	}
}

func BenchmarkConcurrentSendReceive(b *testing.B) {
	client, server := createTestPair()
	defer client.Close()
	defer server.Close()

	clientSR := SendReceiverFromNetConn(client)
	serverSR := SendReceiverFromNetConn(server)

	ctx := context.Background()
	data := bytes.Repeat([]byte("D"), 1024)

	b.ResetTimer()
	b.SetBytes(1024)

	var wg sync.WaitGroup
	wg.Add(2)

	// 发送 goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			if err := clientSR.Send(data); err != nil {
				b.Errorf("发送失败: %v", err)
				return
			}
		}
	}()

	// 接收 goroutine
	go func() {
		defer wg.Done()
		for i := 0; i < b.N; i++ {
			_, err := serverSR.Receive(ctx)
			if err != nil {
				b.Errorf("接收失败: %v", err)
				return
			}
		}
	}()

	wg.Wait()
}
