package flow

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// TestNewControl 测试创建控制链
func TestNewControl(t *testing.T) {
	c := NewControl()
	if c == nil {
		t.Fatal("NewControl returned nil")
	}
	if c.Len() != 0 {
		t.Fatalf("expected 0 handlers, got %d", c.Len())
	}
}

// TestNewControlWithHandlers 测试带 handler 创建控制链
func TestNewControlWithHandlers(t *testing.T) {
	callCount := int32(0)
	handler1 := func(ctx context.Context, data []byte) {
		atomic.AddInt32(&callCount, 1)
	}
	handler2 := func(ctx context.Context, data []byte) {
		atomic.AddInt32(&callCount, 1)
	}

	c := NewControl(handler1, handler2)
	if c.Len() != 2 {
		t.Fatalf("expected 2 handlers, got %d", c.Len())
	}
}

// TestHandleEmpty 测试空控制链的 Handle
func TestHandleEmpty(t *testing.T) {
	c := NewControl()
	data := []byte("test data")
	result := c.Handle(context.Background(), data)

	if result == nil {
		t.Fatal("expected data to be returned")
	}
	if len(result) != len(data) {
		t.Fatalf("expected data length %d, got %d", len(data), len(result))
	}
}

// TestHandleSingleHandler 测试单个 handler 的执行
func TestHandleSingleHandler(t *testing.T) {
	var called bool
	handler := func(ctx context.Context, data []byte) {
		called = true
		if string(data) != "test data" {
			t.Errorf("expected 'test data', got '%s'", string(data))
		}
	}

	c := NewControl(handler)
	data := []byte("test data")
	result := c.Handle(context.Background(), data)

	if !called {
		t.Error("handler was not called")
	}
	if len(result) != len(data) {
		t.Fatalf("expected data length %d, got %d", len(data), len(result))
	}
}

// TestHandleMultipleHandlers 测试多个 handler 的执行顺序
func TestHandleMultipleHandlers(t *testing.T) {
	var executionOrder []int
	handler1 := func(ctx context.Context, data []byte) {
		executionOrder = append(executionOrder, 1)
	}
	handler2 := func(ctx context.Context, data []byte) {
		executionOrder = append(executionOrder, 2)
	}
	handler3 := func(ctx context.Context, data []byte) {
		executionOrder = append(executionOrder, 3)
	}

	c := NewControl(handler1, handler2, handler3)
	c.Handle(context.Background(), []byte("test"))

	if len(executionOrder) != 3 {
		t.Fatalf("expected 3 handlers to be called, got %d", len(executionOrder))
	}
	for i, expected := range []int{1, 2, 3} {
		if executionOrder[i] != expected {
			t.Errorf("expected order %d, got %d at index %d", expected, executionOrder[i], i)
		}
	}
}

// TestHandleModifyData 测试 handler 原地修改数据
func TestHandleModifyData(t *testing.T) {
	handler := func(ctx context.Context, data []byte) {
		if len(data) > 0 {
			data[0] = 'X'
		}
	}

	c := NewControl(handler)
	data := []byte("test")
	result := c.Handle(context.Background(), data)

	if result[0] != 'X' {
		t.Errorf("expected first byte to be 'X', got '%c'", result[0])
	}
	if result[0] != data[0] {
		t.Error("modification should be in-place")
	}
}

// TestHandleNilData 测试处理 nil 数据
func TestHandleNilData(t *testing.T) {
	called := false
	handler := func(ctx context.Context, data []byte) {
		called = true
		if data != nil {
			t.Error("expected nil data, got non-nil")
		}
	}

	c := NewControl(handler)
	result := c.Handle(context.Background(), nil)

	if !called {
		t.Error("handler was not called")
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

// TestHandleContext 测试 context 传递
func TestHandleContext(t *testing.T) {
	type ctxKey struct{}
	key := ctxKey{}
	value := "test value"

	handler := func(ctx context.Context, data []byte) {
		if v := ctx.Value(key); v != value {
			t.Errorf("expected context value %v, got %v", value, v)
		}
	}

	ctx := context.WithValue(context.Background(), key, value)
	c := NewControl(handler)
	c.Handle(ctx, []byte("test"))
}

// TestAddHandler 测试添加 handler
func TestAddHandler(t *testing.T) {
	c := NewControl()
	if c.Len() != 0 {
		t.Fatalf("expected 0 handlers, got %d", c.Len())
	}

	handler := func(ctx context.Context, data []byte) {}
	c.AddHandler(handler)
	if c.Len() != 1 {
		t.Fatalf("expected 1 handler, got %d", c.Len())
	}

	c.AddHandler(handler)
	if c.Len() != 2 {
		t.Fatalf("expected 2 handlers, got %d", c.Len())
	}
}

// TestClear 测试清空 handlers
func TestClear(t *testing.T) {
	handler := func(ctx context.Context, data []byte) {}
	c := NewControl(handler, handler, handler)

	if c.Len() != 3 {
		t.Fatalf("expected 3 handlers, got %d", c.Len())
	}

	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("expected 0 handlers after clear, got %d", c.Len())
	}

	// 测试清空后可以重新添加
	c.AddHandler(handler)
	if c.Len() != 1 {
		t.Fatalf("expected 1 handler after add, got %d", c.Len())
	}
}

// TestLen 测试返回中间件数量
func TestLen(t *testing.T) {
	c := NewControl()
	if c.Len() != 0 {
		t.Fatalf("expected 0 handlers, got %d", c.Len())
	}

	handler := func(ctx context.Context, data []byte) {}

	c.AddHandler(handler)
	if c.Len() != 1 {
		t.Fatalf("expected 1 handler, got %d", c.Len())
	}

	c.AddHandler(handler)
	c.AddHandler(handler)
	if c.Len() != 3 {
		t.Fatalf("expected 3 handlers, got %d", c.Len())
	}
}

// TestNopControl 测试空控制链单例
func TestNopControl(t *testing.T) {
	if NopControl == nil {
		t.Fatal("NopControl is nil")
	}

	// 测试 NopControl 可以安全调用
	result := NopControl.Handle(context.Background(), []byte("test"))
	if len(result) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(result))
	}
}

// TestControlConcurrency 测试并发安全性
func TestControlConcurrency(t *testing.T) {
	var callCount int32

	handler := func(ctx context.Context, data []byte) {
		atomic.AddInt32(&callCount, 1)
	}

	// 创建控制链（非并发添加，避免竞态）
	c := NewControl()
	for i := 0; i < 100; i++ {
		c.AddHandler(handler)
	}

	// 并发执行 Handle（这应该是线程安全的）
	callCount = 0
	data := []byte("test")
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Handle(context.Background(), data)
		}()
	}
	wg.Wait()

	expected := int32(100 * 1000) // 100 handlers * 1000 calls
	if atomic.LoadInt32(&callCount) != expected {
		t.Fatalf("expected %d calls, got %d", expected, callCount)
	}
}

// BenchmarkNewControl 基准测试创建控制链
func BenchmarkNewControl(b *testing.B) {
	handler := func(ctx context.Context, data []byte) {}

	b.Run("Empty", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewControl()
		}
	})

	b.Run("With1Handler", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewControl(handler)
		}
	})

	b.Run("With5Handlers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewControl(handler, handler, handler, handler, handler)
		}
	})

	b.Run("With10Handlers", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewControl(handler, handler, handler, handler, handler,
				handler, handler, handler, handler, handler)
		}
	})
}

// BenchmarkHandle 基准测试 Handle 方法
func BenchmarkHandle(b *testing.B) {
	handler := func(ctx context.Context, data []byte) {
		// 模拟简单处理
		_ = len(data)
	}

	data := []byte("test data for benchmark")
	ctx := context.Background()

	b.Run("EmptyControl", func(b *testing.B) {
		c := NewControl()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Handle(ctx, data)
		}
	})

	b.Run("1Handler", func(b *testing.B) {
		c := NewControl(handler)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Handle(ctx, data)
		}
	})

	b.Run("5Handlers", func(b *testing.B) {
		c := NewControl(handler, handler, handler, handler, handler)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Handle(ctx, data)
		}
	})

	b.Run("10Handlers", func(b *testing.B) {
		c := NewControl(handler, handler, handler, handler, handler,
			handler, handler, handler, handler, handler)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Handle(ctx, data)
		}
	})

	b.Run("50Handlers", func(b *testing.B) {
		handlers := make([]Handler, 50)
		for i := range handlers {
			handlers[i] = handler
		}
		c := NewControl(handlers...)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Handle(ctx, data)
		}
	})
}

// BenchmarkHandleWithDataModification 基准测试带数据修改的 Handle
func BenchmarkHandleWithDataModification(b *testing.B) {
	handler := func(ctx context.Context, data []byte) {
		if len(data) > 0 {
			data[0] = data[0] ^ 0xFF // 简单修改
		}
	}

	data := []byte("test data")
	ctx := context.Background()

	b.Run("1Handler", func(b *testing.B) {
		c := NewControl(handler)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Handle(ctx, data)
		}
	})

	b.Run("10Handlers", func(b *testing.B) {
		c := NewControl(handler, handler, handler, handler, handler,
			handler, handler, handler, handler, handler)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Handle(ctx, data)
		}
	})
}

// BenchmarkAddHandler 基准测试添加 handler
func BenchmarkAddHandler(b *testing.B) {
	handler := func(ctx context.Context, data []byte) {}

	b.Run("EmptyControl", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c := NewControl()
			c.AddHandler(handler)
		}
	})

	b.Run("ExistingControl", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			c := NewControl(handler, handler, handler, handler, handler)
			c.AddHandler(handler)
		}
	})
}

// BenchmarkParallelHandle 并发基准测试
func BenchmarkParallelHandle(b *testing.B) {
	handler := func(ctx context.Context, data []byte) {
		_ = len(data)
	}

	c := NewControl(handler, handler, handler, handler, handler)
	data := []byte("test data")
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Handle(ctx, data)
		}
	})
}
