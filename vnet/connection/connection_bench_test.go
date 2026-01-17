package connection

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

// ========== 性能测试 ==========

// benchmarkReadWriteCloser 用于性能测试的读写实现
type benchmarkReadWriteCloser struct {
	reader *bytes.Reader
	writer *bytes.Buffer
}

func newBenchmarkReadWriteCloser(data []byte) *benchmarkReadWriteCloser {
	return &benchmarkReadWriteCloser{
		reader: bytes.NewReader(data),
		writer: &bytes.Buffer{},
	}
}

func (b *benchmarkReadWriteCloser) Read(p []byte) (n int, err error) {
	return b.reader.Read(p)
}

func (b *benchmarkReadWriteCloser) Write(p []byte) (n int, err error) {
	return b.writer.Write(p)
}

func (b *benchmarkReadWriteCloser) Close() error {
	return nil
}

// BenchmarkNewConnection 测试创建连接的性能
func BenchmarkNewConnection(b *testing.B) {
	ctx := context.Background()
	mock := &MockAdaptor{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewConnection(ctx, mock)
	}
}

// BenchmarkMustDial 测试 dial 的性能
func BenchmarkMustDial(b *testing.B) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()
	}
}

// BenchmarkMustDial_OnlyOnce 测试重复 dial 的性能（只实际 dial 一次）
func BenchmarkMustDial_OnlyOnce(b *testing.B) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
	}
	conn := NewConnection(ctx, mock)
	_ = conn.MustDial() // 第一次 dial

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = conn.MustDial() // 后续 dial，rwc 不为 nil
	}
}

// BenchmarkRead_SmallPacket 测试小包读取性能
func BenchmarkRead_SmallPacket(b *testing.B) {
	ctx := context.Background()
	data := []byte("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
		}
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()

		buf := make([]byte, len(data))
		_, _ = conn.Read(buf)
	}
}

// BenchmarkRead_MediumPacket 测试中等包读取性能
func BenchmarkRead_MediumPacket(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 1024) // 1KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
		}
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()

		buf := make([]byte, len(data))
		_, _ = conn.Read(buf)
	}
}

// BenchmarkRead_LargePacket 测试大包读取性能
func BenchmarkRead_LargePacket(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 64*1024) // 64KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
		}
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()

		buf := make([]byte, len(data))
		_, _ = conn.Read(buf)
	}
}

// BenchmarkWrite_SmallPacket 测试小包写入性能
func BenchmarkWrite_SmallPacket(b *testing.B) {
	ctx := context.Background()
	data := []byte("test")
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
	}
	conn := NewConnection(ctx, mock)
	_ = conn.MustDial()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = conn.Write(data)
	}
}

// BenchmarkWrite_MediumPacket 测试中等包写入性能
func BenchmarkWrite_MediumPacket(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 1024) // 1KB
	for i := range data {
		data[i] = byte(i % 256)
	}
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
	}
	conn := NewConnection(ctx, mock)
	_ = conn.MustDial()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = conn.Write(data)
	}
}

// BenchmarkWrite_LargePacket 测试大包写入性能
func BenchmarkWrite_LargePacket(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 64*1024) // 64KB
	for i := range data {
		data[i] = byte(i % 256)
	}
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
	}
	conn := NewConnection(ctx, mock)
	_ = conn.MustDial()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = conn.Write(data)
	}
}

// BenchmarkReadWrite_Pair 测试读写对性能
func BenchmarkReadWrite_Pair(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 512) // 512B
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
		}
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()

		readBuf := make([]byte, len(data))
		_, _ = conn.Read(readBuf)
		_, _ = conn.Write(data)
	}
}

// BenchmarkOnRead_Simple 测试简单 OnRead 的性能开销
func BenchmarkOnRead_Simple(b *testing.B) {
	ctx := context.Background()
	data := []byte("test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
			onReadFunc: func(p []byte) []byte {
				return p // 直接返回，不做任何处理
			},
		}
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()

		buf := make([]byte, len(data))
		_, _ = conn.Read(buf)
	}
}

// BenchmarkOnRead_Transform 测试带转换的 OnRead 性能
func BenchmarkOnRead_Transform(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
			onReadFunc: func(p []byte) []byte {
				// 模拟简单的数据转换
				result := make([]byte, len(p))
				for i, b := range p {
					result[i] = b + 1
				}
				return result
			},
		}
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()

		buf := make([]byte, len(data))
		_, _ = conn.Read(buf)
	}
}

// BenchmarkOnWrite_Simple 测试简单 OnWrite 的性能开销
func BenchmarkOnWrite_Simple(b *testing.B) {
	ctx := context.Background()
	data := []byte("test data")
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
		onWriteFunc: func(p []byte) []byte {
			return p // 直接返回，不做任何处理
		},
	}
	conn := NewConnection(ctx, mock)
	_ = conn.MustDial()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = conn.Write(data)
	}
}

// BenchmarkOnWrite_Transform 测试带转换的 OnWrite 性能
func BenchmarkOnWrite_Transform(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
		onWriteFunc: func(p []byte) []byte {
			// 模拟简单的数据转换
			result := make([]byte, len(p))
			for i, b := range p {
				result[i] = b + 1
			}
			return result
		},
	}
	conn := NewConnection(ctx, mock)
	_ = conn.MustDial()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = conn.Write(data)
	}
}

// BenchmarkClose 测试关闭连接的性能
func BenchmarkClose(b *testing.B) {
	ctx := context.Background()
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()
		_ = conn.Close()
	}
}

// BenchmarkConcurrentRead 测试并发读取性能
func BenchmarkConcurrentRead(b *testing.B) {
	ctx := context.Background()
	data := bytes.Repeat([]byte("test"), 100) // 400 bytes
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser(data), nil
		},
	}
	conn := NewConnection(ctx, mock)
	_ = conn.MustDial()

	buf := make([]byte, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.rwc = newBenchmarkReadWriteCloser(data)
		for j := 0; j < 10; j++ {
			_, _ = conn.Read(buf)
		}
	}
}

// BenchmarkConcurrentWrite 测试并发写入性能
func BenchmarkConcurrentWrite(b *testing.B) {
	ctx := context.Background()
	data := []byte("test")
	mock := &MockAdaptor{
		dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
			return newBenchmarkReadWriteCloser([]byte{}), nil
		},
	}
	conn := NewConnection(ctx, mock)
	_ = conn.MustDial()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 100; j++ {
			_, _ = conn.Write(data)
		}
	}
}

// BenchmarkError_Creation 测试错误创建的性能
func BenchmarkError_Creation(b *testing.B) {
	ctx := context.Background()
	mock := &MockAdaptor{}
	conn := NewConnection(ctx, mock)

	testErr := errors.New("test error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := &Error{
			Conn:     conn,
			Position: PosRead,
			Err:      testErr,
		}
		_ = err.Error()
	}
}

// BenchmarkFullWorkflow 测试完整工作流程的性能
// 包括创建、dial、读取、写入、关闭
func BenchmarkFullWorkflow(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
		}
		conn := NewConnection(ctx, mock)

		_ = conn.MustDial()

		buf := make([]byte, len(data))
		_, _ = conn.Read(buf)
		_, _ = conn.Write(data)
		_ = conn.Close()
	}
}

// BenchmarkAutoDial_Read 测试读取时自动 dial 的性能
func BenchmarkAutoDial_Read(b *testing.B) {
	ctx := context.Background()
	data := []byte("test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
		}
		conn := NewConnection(ctx, mock)

		buf := make([]byte, len(data))
		_, _ = conn.Read(buf)
	}
}

// BenchmarkAutoDial_Write 测试写入时自动 dial 的性能
func BenchmarkAutoDial_Write(b *testing.B) {
	ctx := context.Background()
	data := []byte("test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser([]byte{}), nil
			},
		}
		conn := NewConnection(ctx, mock)

		_, _ = conn.Write(data)
	}
}

// BenchmarkMemory_Allocation 测试内存分配情况
func BenchmarkMemory_Allocation(b *testing.B) {
	ctx := context.Background()
	data := make([]byte, 4096) // 4KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mock := &MockAdaptor{
			dialFunc: func(ctx context.Context) (io.ReadWriteCloser, error) {
				return newBenchmarkReadWriteCloser(data), nil
			},
		}
		conn := NewConnection(ctx, mock)
		_ = conn.MustDial()

		buf := make([]byte, len(data))
		_, _ = conn.Read(buf)
		_, _ = conn.Write(data)
	}
}
