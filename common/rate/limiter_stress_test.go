package rate

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// StressTestResult 压力测试结果
type StressTestResult struct {
	TotalRequests   int64
	AllowedRequests int64
	BlockedRequests int64
	Duration        time.Duration
	Throughput      float64       // QPS
	Latency         time.Duration // 平均延迟
	MaxConcurrency  int
}

// BenchmarkStress 无锁限流器压力测试
func BenchmarkStress(b *testing.B) {
	testCases := []struct {
		name                 string
		rate                 int
		burst                int
		concurrency          int
		requestsPerGoroutine int
	}{
		{"LowRate", 100, 100, 10, 1000},
		{"MediumRate", 1000, 1000, 100, 10000},
		{"HighRate", 10000, 10000, 100, 10000},
		{"BurstTest", 100, 10000, 50, 5000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			result := runStressTest(&Limiter{}, tc.rate, tc.burst, tc.concurrency, tc.requestsPerGoroutine)
			printResult("Lockfree", tc.name, result)
		})
	}
}

// BenchmarkStdLimiterStress 标准库压力测试
func BenchmarkStdLimiterStress(b *testing.B) {
	testCases := []struct {
		name                 string
		rate                 int
		burst                int
		concurrency          int
		requestsPerGoroutine int
	}{
		{"LowRate", 100, 100, 10, 1000},
		{"MediumRate", 1000, 1000, 100, 10000},
		{"HighRate", 10000, 10000, 100, 10000},
		{"BurstTest", 100, 10000, 50, 5000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			result := runStdLimiterStressTest(tc.rate, tc.burst, tc.concurrency, tc.requestsPerGoroutine)
			printResult("StdLib", tc.name, result)
		})
	}
}

// runStressTest 运行无锁限流器压力测试
func runStressTest(limiter *Limiter, limiterRate, burst, concurrency, requestsPerGoroutine int) *StressTestResult {
	limiter = NewLimiter(limiterRate, burst)

	var (
		wg           sync.WaitGroup
		allowed      atomic.Int64
		blocked      atomic.Int64
		totalLatency atomic.Int64
	)

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				reqStart := time.Now()
				result := limiter.Allow()
				latency := time.Since(reqStart)

				if result {
					allowed.Add(1)
				} else {
					blocked.Add(1)
				}
				totalLatency.Add(int64(latency))
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	total := allowed.Load() + blocked.Load()

	return &StressTestResult{
		TotalRequests:   total,
		AllowedRequests: allowed.Load(),
		BlockedRequests: blocked.Load(),
		Duration:        duration,
		Throughput:      float64(total) / duration.Seconds(),
		Latency:         time.Duration(totalLatency.Load() / total),
		MaxConcurrency:  concurrency,
	}
}

// runStdLimiterStressTest 运行标准库压力测试
func runStdLimiterStressTest(limiterRate, burst, concurrency, requestsPerGoroutine int) *StressTestResult {
	limiter := rate.NewLimiter(rate.Limit(limiterRate), burst)

	var (
		wg           sync.WaitGroup
		allowed      atomic.Int64
		blocked      atomic.Int64
		totalLatency atomic.Int64
	)

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				reqStart := time.Now()
				result := limiter.Allow()
				latency := time.Since(reqStart)

				if result {
					allowed.Add(1)
				} else {
					blocked.Add(1)
				}
				totalLatency.Add(int64(latency))
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	total := allowed.Load() + blocked.Load()

	return &StressTestResult{
		TotalRequests:   total,
		AllowedRequests: allowed.Load(),
		BlockedRequests: blocked.Load(),
		Duration:        duration,
		Throughput:      float64(total) / duration.Seconds(),
		Latency:         time.Duration(totalLatency.Load() / total),
		MaxConcurrency:  concurrency,
	}
}

// printResult 打印测试结果
func printResult(limiterType, testName string, result *StressTestResult) {
	fmt.Printf("\n=== %s Limiter: %s ===\n", limiterType, testName)
	fmt.Printf("Total Requests:    %d\n", result.TotalRequests)
	fmt.Printf("Allowed Requests:  %d\n", result.AllowedRequests)
	fmt.Printf("Blocked Requests:  %d\n", result.BlockedRequests)
	fmt.Printf("Duration:          %v\n", result.Duration)
	fmt.Printf("Throughput:        %.2f QPS\n", result.Throughput)
	fmt.Printf("Avg Latency:       %v\n", result.Latency)
	fmt.Printf("Max Concurrency:   %d\n", result.MaxConcurrency)
}

// TestStressComparison 对比压力测试
func TestStressComparison(t *testing.T) {
	testCases := []struct {
		name                 string
		rate                 int
		burst                int
		concurrency          int
		requestsPerGoroutine int
	}{
		{"Case1-LowRate", 100, 100, 10, 1000},
		{"Case2-MediumRate", 1000, 1000, 100, 10000},
		{"Case3-HighRate", 10000, 10000, 100, 10000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("\n========== %s ==========\n", tc.name)

			// 测试无锁限流器
			lockfreeResult := runStressTest(nil, tc.rate, tc.burst, tc.concurrency, tc.requestsPerGoroutine)

			// 让系统休息一下
			time.Sleep(time.Second)

			// 测试标准库
			stdResult := runStdLimiterStressTest(tc.rate, tc.burst, tc.concurrency, tc.requestsPerGoroutine)

			// 对比结果
			fmt.Printf("\n=== Comparison ===\n")
			throughputImprovement := (lockfreeResult.Throughput - stdResult.Throughput) / stdResult.Throughput * 100
			latencyImprovement := (float64(stdResult.Latency) - float64(lockfreeResult.Latency)) / float64(stdResult.Latency) * 100

			fmt.Printf("Throughput Improvement: %.2f%%\n", throughputImprovement)
			fmt.Printf("Latency Improvement:    %.2f%%\n", latencyImprovement)
		})
	}
}

// BenchmarkMemoryAlloc 内存分配对比
func BenchmarkMemoryAlloc(b *testing.B) {
	b.Run("Lockfree", func(b *testing.B) {
		limiter := NewLimiter(10000, 10000)
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			limiter.Allow()
		}
	})

	b.Run("StdLib", func(b *testing.B) {
		limiter := rate.NewLimiter(rate.Limit(10000), 10000)
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			limiter.Allow()
		}
	})
}

// BenchmarkGoroutineScalability Goroutine扩展性测试
func BenchmarkGoroutineScalability(b *testing.B) {
	concurrencyLevels := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Lockfree-%d", concurrency), func(b *testing.B) {
			limiter := NewLimiter(1000000, 1000000)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					limiter.Allow()
				}
			})
		})
	}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("StdLib-%d", concurrency), func(b *testing.B) {
			limiter := rate.NewLimiter(rate.Limit(1000000), 1000000)
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					limiter.Allow()
				}
			})
		})
	}
}

// BenchmarkLongRunning 长时间运行测试
func BenchmarkLongRunning(b *testing.B) {
	duration := 10 * time.Second

	b.Run("Lockfree", func(b *testing.B) {
		limiter := NewLimiter(10000, 10000)
		var counter atomic.Int64

		endTime := time.Now().Add(duration)
		b.ResetTimer()

		for time.Now().Before(endTime) {
			if limiter.Allow() {
				counter.Add(1)
			}
		}

		elapsed := duration.Seconds()
		qps := float64(counter.Load()) / elapsed
		b.ReportMetric(qps, "qps")
	})

	b.Run("StdLib", func(b *testing.B) {
		limiter := rate.NewLimiter(rate.Limit(10000), 10000)
		var counter atomic.Int64

		endTime := time.Now().Add(duration)
		b.ResetTimer()

		for time.Now().Before(endTime) {
			if limiter.Allow() {
				counter.Add(1)
			}
		}

		elapsed := duration.Seconds()
		qps := float64(counter.Load()) / elapsed
		b.ReportMetric(qps, "qps")
	})
}

// BenchmarkContention 争用测试
func BenchmarkContention(b *testing.B) {
	// 低速率以制造争用
	b.Run("Lockfree-HighContention", func(b *testing.B) {
		limiter := NewLimiter(100, 100)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				limiter.Allow()
			}
		})
	})

	b.Run("StdLib-HighContention", func(b *testing.B) {
		limiter := rate.NewLimiter(rate.Limit(100), 100)
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				limiter.Allow()
			}
		})
	})
}

// TestRealWorldScenario 真实场景模拟测试
func TestRealWorldScenario(t *testing.T) {
	// 模拟API限流场景：发送超过突发容量的请求
	scenarios := []struct {
		name     string
		rate     int
		burst    int
		requests int
		interval time.Duration // 请求间隔
	}{
		{"API-100QPS", 100, 200, 300, 3 * time.Millisecond},       // 300个请求，间隔3ms = 333 QPS，速率100 QPS
		{"API-1000QPS", 1000, 2000, 3000, 300 * time.Microsecond}, // 3000个请求，间隔300us = 3333 QPS，速率1000 QPS
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			limiter := NewLimiter(scenario.rate, scenario.burst)
			start := time.Now()
			allowed := 0

			for i := 0; i < scenario.requests; i++ {
				if limiter.Allow() {
					allowed++
				}
				time.Sleep(scenario.interval)
			}

			elapsed := time.Since(start).Seconds()
			t.Logf("Test %s: Allowed %d/%d requests in %.2fs", scenario.name, allowed, scenario.requests, elapsed)

			// 验证允许的请求数在合理范围内
			// 理论最大值 = 突发容量 + 速率 * 时间
			theoreticalMax := float64(scenario.burst) + float64(scenario.rate)*elapsed
			if float64(allowed) > theoreticalMax*1.2 { // 允许20%的误差
				t.Errorf("Allowed %d requests exceeds theoretical max %.0f (burst %d + rate %d * time %.2fs)",
					allowed, theoreticalMax, scenario.burst, scenario.rate, elapsed)
			}

			// 验证至少允许了突发容量
			if allowed < scenario.burst*9/10 { // 允许10%的误差
				t.Errorf("Allowed %d requests should be at least close to burst capacity %d", allowed, scenario.burst)
			}
		})
	}
}

// TestLockfreeLimiter_NoDeadlock 死锁检测测试
func TestLockfreeLimiter_NoDeadlock(t *testing.T) {
	limiter := NewLimiter(100, 100)
	done := make(chan bool)

	// 启动多个goroutine
	for i := 0; i < 100; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				limiter.Allow()
				limiter.AllowN(10)
				limiter.Tokens()
			}
			done <- true
		}()
	}

	// 等待所有goroutine完成，设置超时防止死锁
	timeout := time.After(10 * time.Second)
	completed := 0

	for {
		select {
		case <-done:
			completed++
			if completed == 100 {
				return // 所有goroutine都完成了
			}
		case <-timeout:
			t.Fatal("Test timeout - possible deadlock detected")
		}
	}
}

// TestCPUProfiling CPU使用率对比（手动测试用）
func TestCPUProfiling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CPU profiling test in short mode")
	}

	duration := 5 * time.Second

	t.Run("Lockfree", func(t *testing.T) {
		limiter := NewLimiter(100000, 100000)
		endTime := time.Now().Add(duration)

		for time.Now().Before(endTime) {
			limiter.Allow()
		}
	})

	t.Run("StdLib", func(t *testing.T) {
		limiter := rate.NewLimiter(rate.Limit(100000), 100000)
		endTime := time.Now().Add(duration)

		for time.Now().Before(endTime) {
			limiter.Allow()
		}
	})
}

// PrintGOMAXPROCS 打印GOMAXPROCS信息
func init() {
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
}
