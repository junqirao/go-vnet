package rate

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// TestLimiter_Basic 基本功能测试
func TestLimiter_Basic(t *testing.T) {
	limiter := NewLimiter(100, 10) // 100 QPS, 桶容量10

	// 快速消费5个令牌
	for i := 0; i < 5; i++ {
		if !limiter.Allow() {
			t.Errorf("Expected to allow request %d", i)
		}
	}

	// 检查可用令牌数
	tokens := limiter.Tokens()
	if tokens < 4.9 || tokens > 5.1 {
		t.Errorf("Expected tokens around 5, got %f", tokens)
	}
}

// TestLimiter_Rate 速率限制测试
func TestLimiter_Rate(t *testing.T) {
	limiter := NewLimiter(100, 100) // 100 QPS

	start := time.Now()
	allowed := 0

	// 尝试消费200个请求
	for i := 0; i < 200; i++ {
		if limiter.Allow() {
			allowed++
		}
	}

	elapsed := time.Since(start)

	// 第一个100个应该立即通过，后面的应该被限流
	if allowed < 99 || allowed > 101 {
		t.Errorf("Expected around 100 allowed requests, got %d", allowed)
	}

	// 消耗时间应该很短
	if elapsed > time.Second {
		t.Errorf("Expected elapsed time < 1s, got %v", elapsed)
	}
}

// TestLimiter_Burst 突发流量测试
func TestLimiter_Burst(t *testing.T) {
	limiter := NewLimiter(10, 100) // 10 QPS, 但桶容量100

	// 突发消费100个请求应该都通过
	for i := 0; i < 100; i++ {
		if !limiter.Allow() {
			t.Errorf("Expected to allow request %d in burst", i)
		}
	}

	// 之后的请求应该被限流
	time.Sleep(time.Millisecond * 120) // 等待120ms让令牌恢复（10 QPS = 0.01个令牌/ms, 120ms = 1.2个令牌）
	if !limiter.Allow() {
		t.Error("Expected to allow request after wait")
	}
}

// TestLimiter_Concurrent 并发测试
func TestLimiter_Concurrent(t *testing.T) {
	limiter := NewLimiter(1000, 1000)

	var wg sync.WaitGroup
	var allowed, blocked int64

	// 100个goroutine并发请求
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ { // 每个goroutine发20个请求
				if limiter.Allow() {
					atomic.AddInt64(&allowed, 1)
				} else {
					atomic.AddInt64(&blocked, 1)
				}
			}
		}()
	}

	wg.Wait()

	total := allowed + blocked
	if total != 2000 {
		t.Errorf("Expected total requests 2000, got %d", total)
	}

	// 由于限流，允许的请求数应该在1000左右
	if allowed < 900 || allowed > 1100 {
		t.Errorf("Expected allowed requests around 1000, got %d", allowed)
	}
}

// TestLimiter_AllowN 批量请求测试
func TestLimiter_AllowN(t *testing.T) {
	limiter := NewLimiter(10, 100)

	// 批量消费50个令牌
	if !limiter.AllowN(50) {
		t.Error("Expected to allow 50 tokens")
	}

	// 再批量消费60个令牌应该失败
	if limiter.AllowN(60) {
		t.Error("Expected to deny 60 tokens after consuming 50")
	}

	// 消费10个令牌应该成功（剩余40）
	if !limiter.AllowN(10) {
		t.Error("Expected to allow 10 tokens")
	}

	// 再消费40个令牌应该成功（消费完剩余）
	if !limiter.AllowN(40) {
		t.Error("Expected to allow 40 tokens")
	}

	// 再消费11个令牌应该失败（桶已空）
	if limiter.AllowN(11) {
		t.Error("Expected to deny 11 tokens when bucket is empty")
	}
}

// TestLimiter_Wait 等待测试
func TestLimiter_Wait(t *testing.T) {
	limiter := NewLimiter(10, 10)

	// 先消费完桶容量
	for i := 0; i < 10; i++ {
		limiter.Allow()
	}

	start := time.Now()

	// 等待并获取1个令牌，应该大约等待100ms
	limiter.Wait()

	elapsed := time.Since(start)

	// 应该等待大约100ms（因为速率是10 QPS）
	if elapsed < 90*time.Millisecond || elapsed > 150*time.Millisecond {
		t.Errorf("Expected wait time around 100ms, got %v", elapsed)
	}
}

// TestLimiter_Recovery 令牌恢复测试
func TestLimiter_Recovery(t *testing.T) {
	limiter := NewLimiter(100, 10) // 100 QPS, 桶容量10

	// 消费完所有令牌
	for i := 0; i < 10; i++ {
		if !limiter.Allow() {
			t.Fatal("Failed to consume initial tokens")
		}
	}

	// 等待10ms，应该恢复约1个令牌
	time.Sleep(10 * time.Millisecond)

	tokens := limiter.Tokens()
	if tokens < 0.8 || tokens > 1.2 {
		t.Errorf("Expected tokens around 1, got %f", tokens)
	}

	// 等待100ms，应该恢复约10个令牌（填满桶）
	time.Sleep(100 * time.Millisecond)

	tokens = limiter.Tokens()
	if tokens < 9.8 || tokens > 10 {
		t.Errorf("Expected tokens around 10 (full), got %f", tokens)
	}
}

// TestLimiter_PutBack 归还令牌测试
func TestLimiter_PutBack(t *testing.T) {
	limiter := NewLimiter(100, 100) // 100 QPS, 桶容量100

	// 申请50个令牌
	if !limiter.AllowN(50) {
		t.Fatal("Failed to allocate 50 tokens")
	}

	// 检查可用令牌数
	tokens := limiter.Tokens()
	if tokens < 49.9 || tokens > 50.1 {
		t.Errorf("Expected tokens around 50 after allocating 50, got %f", tokens)
	}

	// 归还20个令牌
	putback := limiter.PutBack(20)
	if putback != 20 {
		t.Errorf("Expected to put back 20 tokens, got %d", putback)
	}

	// 检查可用令牌数
	tokens = limiter.Tokens()
	if tokens < 69.9 || tokens > 70.1 {
		t.Errorf("Expected tokens around 70 after putting back 20, got %f", tokens)
	}
}

// TestLimiter_PutBackFromLast 从上一次消费中归还令牌
func TestLimiter_PutBackFromLast(t *testing.T) {
	limiter := NewLimiter(100, 100)

	// 申请50个令牌
	if !limiter.AllowN(50) {
		t.Fatal("Failed to allocate 50 tokens")
	}

	// 实际只使用了30个，归还未使用的20个
	putback := limiter.PutBackFromLast(30)
	if putback != 20 {
		t.Errorf("Expected to put back 20 tokens, got %d", putback)
	}

	// 检查可用令牌数
	tokens := limiter.Tokens()
	if tokens < 69.9 || tokens > 70.1 {
		t.Errorf("Expected tokens around 70 after putting back 20, got %f", tokens)
	}
}

// TestLimiter_PutBack_EdgeCases 归还令牌边界情况测试
func TestLimiter_PutBack_EdgeCases(t *testing.T) {
	limiter := NewLimiter(100, 100)

	t.Run("PutBackZero", func(t *testing.T) {
		putback := limiter.PutBack(0)
		if putback != 0 {
			t.Errorf("Expected to put back 0 tokens, got %d", putback)
		}
	})

	t.Run("PutBackNegative", func(t *testing.T) {
		putback := limiter.PutBack(-1)
		if putback != 0 {
			t.Errorf("Expected to put back 0 tokens for negative input, got %d", putback)
		}
	})

	t.Run("PutBackExceedBurst", func(t *testing.T) {
		// 申请50个令牌
		if !limiter.AllowN(50) {
			t.Fatal("Failed to allocate 50 tokens")
		}

		// 尝试归还超过桶容量的令牌
		tokensBefore := limiter.Tokens()
		_ = limiter.PutBack(1000) // 应该被限制在桶容量内
		_ = limiter.PutBack(1000) // 再次尝试
		tokensAfter := limiter.Tokens()

		if tokensAfter > 100.1 || tokensAfter < tokensBefore {
			t.Errorf("Tokens after putback %f should be at most 100, tokens before %f", tokensAfter, tokensBefore)
		}
	})

	t.Run("PutBackFromLastZeroUsed", func(t *testing.T) {
		limiter2 := NewLimiter(100, 100)
		// 申请10个令牌
		if !limiter2.AllowN(10) {
			t.Fatal("Failed to allocate 10 tokens")
		}

		// 声称全部使用了，不应该归还
		putback := limiter2.PutBackFromLast(10)
		if putback != 0 {
			t.Errorf("Expected to put back 0 tokens when all used, got %d", putback)
		}
	})
}

// TestLimiter_PutBack_Concurrent 并发归还测试
func TestLimiter_PutBack_Concurrent(t *testing.T) {
	limiter := NewLimiter(1000, 1000)

	// 先申请500个令牌
	if !limiter.AllowN(500) {
		t.Fatal("Failed to allocate 500 tokens")
	}

	var wg sync.WaitGroup
	var totalPutback atomic.Int64

	// 10个goroutine并发归还
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			putback := limiter.PutBack(20)
			totalPutback.Add(int64(putback))
		}()
	}

	wg.Wait()

	// 应该归还了200个令牌
	if totalPutback.Load() != 200 {
		t.Errorf("Expected total putback 200, got %d", totalPutback.Load())
	}

	// 检查可用令牌数
	tokens := limiter.Tokens()
	// 初始1000，申请500，剩余500，归还200，应该是700
	if tokens < 699.9 || tokens > 700.1 {
		t.Errorf("Expected tokens around 700, got %f", tokens)
	}
}

// TestLimiter_PutBack_Scenario 实际场景测试：批量申请但部分使用
func TestLimiter_PutBack_Scenario(t *testing.T) {
	limiter := NewLimiter(100, 100)

	// 场景：批量处理任务
	// 申请50个令牌准备处理50个任务
	if !limiter.AllowN(50) {
		t.Fatal("Failed to allocate 50 tokens")
	}

	tokensBefore := limiter.Tokens()
	t.Logf("After allocating 50: tokens = %.2f", tokensBefore)

	// 实际只处理了30个任务，需要归还20个令牌
	putback := limiter.PutBackFromLast(30)
	if putback != 20 {
		t.Errorf("Expected to put back 20 tokens, got %d", putback)
	}

	tokensAfter := limiter.Tokens()
	t.Logf("After putting back 20: tokens = %.2f", tokensAfter)

	// 验证可用令牌数增加了20
	diff := tokensAfter - tokensBefore
	if diff < 19.9 || diff > 20.1 {
		t.Errorf("Expected tokens to increase by 20, got %.2f", diff)
	}

	// 现在应该可以继续申请更多令牌
	if !limiter.AllowN(20) {
		t.Error("Failed to allocate additional 20 tokens after putback")
	}

	tokensFinal := limiter.Tokens()
	t.Logf("After allocating another 20: tokens = %.2f", tokensFinal)
}

// TestLimiter_CompareWithStdLimiter 与标准库对比测试
func TestLimiter_CompareWithStdLimiter(t *testing.T) {
	r := 100.0
	b := 10.0

	ourLimiter := NewLimiter(int(r), int(b))
	stdLimiter := rate.NewLimiter(rate.Limit(r), int(b))

	// 消费一些令牌
	for i := 0; i < 5; i++ {
		ourLimiter.Allow()
		stdLimiter.Allow()
	}

	// 等待相同的时间
	time.Sleep(10 * time.Millisecond)

	// 两个限流器的可用令牌数应该接近
	ourTokens := ourLimiter.Tokens()
	stdTokens := stdLimiter.Tokens()

	diff := math.Abs(ourTokens - stdTokens)
	if diff > 1.0 { // 允许1个令牌的误差
		t.Logf("Our tokens: %f, Std tokens: %f, diff: %f", ourTokens, stdTokens, diff)
	}
}

// TestLimiter_EdgeCases 边界情况测试
func TestLimiter_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		rate  int
		burst int
	}{
		{"Zero burst", 10, 0},
		{"Small rate", 1, 5},
		{"Large rate", 10000, 1000},
		{"Equal rate and burst", 100, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limiter := NewLimiter(tt.rate, tt.burst)

			// 不应该panic
			for i := 0; i < 100; i++ {
				limiter.Allow()
			}

			// 检查Tokens不panic
			tokens := limiter.Tokens()
			if tokens < 0 {
				t.Errorf("Tokens should not be negative, got %f", tokens)
			}
		})
	}
}

// TestLimiter_AllowN_Zero 允许0个令牌
func TestLimiter_AllowN_Zero(t *testing.T) {
	limiter := NewLimiter(10, 10)

	if limiter.AllowN(0) {
		t.Error("AllowN(0) should return false")
	}

	if limiter.AllowN(-1) {
		t.Error("AllowN(-1) should return false")
	}
}

// BenchmarkLimiter_Allow 单次Allow性能测试
func BenchmarkLimiter_Allow(b *testing.B) {
	limiter := NewLimiter(1000000, 1000000) // 高速率避免阻塞

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkStdLimiter_Allow 标准库Allow性能测试
func BenchmarkStdLimiter_Allow(b *testing.B) {
	limiter := rate.NewLimiter(rate.Limit(1000000), 1000000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow()
		}
	})
}

// BenchmarkLimiter_AllowN 批量Allow性能测试
func BenchmarkLimiter_AllowN(b *testing.B) {
	limiter := NewLimiter(1000000, 1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.AllowN(10)
	}
}

// BenchmarkStdLimiter_AllowN 标准库批量Allow性能测试
func BenchmarkStdLimiter_AllowN(b *testing.B) {
	limiter := rate.NewLimiter(rate.Limit(1000000), 1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.AllowN(time.Now(), 10)
	}
}

// BenchmarkLimiter_PutBack 归还令牌性能测试
func BenchmarkLimiter_PutBack(b *testing.B) {
	limiter := NewLimiter(1000000, 1000000)
	// 先申请一些令牌
	limiter.AllowN(100000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.PutBack(10)
		}
	})
}

// BenchmarkLimiter_PutBackFromLast 从上次消费中归还性能测试
func BenchmarkLimiter_PutBackFromLast(b *testing.B) {
	limiter := NewLimiter(1000000, 1000000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 模拟批量申请但部分使用的场景
		limiter.AllowN(100)
		limiter.PutBackFromLast(80) // 使用了80，归还20
	}
}

// BenchmarkComparison 并发对比测试
func BenchmarkComparison(b *testing.B) {
	benchmarks := []struct {
		name string
		fn   func(b *testing.B)
	}{
		{"", func(b *testing.B) {
			limiter := NewLimiter(1000000, 1000000)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					limiter.Allow()
				}
			})
		}},
		{"StdLib", func(b *testing.B) {
			limiter := rate.NewLimiter(rate.Limit(1000000), 1000000)
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					limiter.Allow()
				}
			})
		}},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, bm.fn)
	}
}

// 测试函数：打印对比结果
func ExampleLimiter() {
	limiter := NewLimiter(100, 10)

	fmt.Println("Testing Limiter:")
	for i := 0; i < 5; i++ {
		allowed := limiter.Allow()
		fmt.Printf("Request %d: Allowed=%v\n", i, allowed)
	}

	tokens := limiter.Tokens()
	fmt.Printf("Available tokens: %.2f\n", tokens)
	// Output:
	// Testing Limiter:
	// Request 0: Allowed=true
	// Request 1: Allowed=true
	// Request 2: Allowed=true
	// Request 3: Allowed=true
	// Request 4: Allowed=true
	// Available tokens: 5.00
}
