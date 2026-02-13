package metrics

import (
	"sync"
	"sync/atomic"
	"testing"
)

// BenchmarkBytesCounter_SingleGoroutine 测试单个 goroutine 的增加操作性能
func BenchmarkBytesCounter_SingleGoroutine(b *testing.B) {
	counter := NewBytesCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Add(1)
	}
}

// BenchmarkBytesCounter_Parallel_1 并发增加测试 - 1个 goroutine
func BenchmarkBytesCounter_Parallel_1(b *testing.B) {
	counter := NewBytesCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
		}
	})
}

// BenchmarkBytesCounter_Parallel_4 并发增加测试 - 4个 goroutine
func BenchmarkBytesCounter_Parallel_4(b *testing.B) {
	counter := NewBytesCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
		}
	})
}

// BenchmarkBytesCounter_Parallel_8 并发增加测试 - 8个 goroutine
func BenchmarkBytesCounter_Parallel_8(b *testing.B) {
	counter := NewBytesCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
		}
	})
}

// BenchmarkBytesCounter_Parallel_16 并发增加测试 - 16个 goroutine
func BenchmarkBytesCounter_Parallel_16(b *testing.B) {
	counter := NewBytesCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
		}
	})
}

// BenchmarkBytesCounter_Parallel_32 并发增加测试 - 32个 goroutine
func BenchmarkBytesCounter_Parallel_32(b *testing.B) {
	counter := NewBytesCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
		}
	})
}

// BenchmarkBytesCounter_Parallel_64 并发增加测试 - 64个 goroutine
func BenchmarkBytesCounter_Parallel_64(b *testing.B) {
	counter := NewBytesCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
		}
	})
}

// BenchmarkBytesCounter_LargeIncrement 测试大数值增加性能
func BenchmarkBytesCounter_LargeIncrement(b *testing.B) {
	counter := NewBytesCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Add(1024) // 增加 1KB
	}
}

// BenchmarkBytesCounter_Get 测试读取性能
func BenchmarkBytesCounter_Get(b *testing.B) {
	counter := NewBytesCounter()
	counter.Add(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Get()
	}
}

// BenchmarkBytesCounter_UpdateStats 测试更新统计性能
func BenchmarkBytesCounter_UpdateStats(b *testing.B) {
	counter := NewBytesCounter()
	counter.Add(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.UpdateStats()
	}
}

// BenchmarkBytesCounter_AddAndGet 测试增加和读取混合操作性能
func BenchmarkBytesCounter_AddAndGet(b *testing.B) {
	counter := NewBytesCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Add(1)
		counter.Get()
	}
}

// BenchmarkBytesCounter_AddAndUpdateStats 测试增加和更新统计混合操作性能
func BenchmarkBytesCounter_AddAndUpdateStats(b *testing.B) {
	counter := NewBytesCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Add(1)
		if i%100 == 0 {
			counter.UpdateStats()
		}
	}
}

// BenchmarkPacketCounter_SingleGoroutine 测试单个 goroutine 的包计数器增加性能
func BenchmarkPacketCounter_SingleGoroutine(b *testing.B) {
	counter := NewPacketCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Add(1)
	}
}

// BenchmarkPacketCounter_Parallel_8 并发增加测试 - 8个 goroutine
func BenchmarkPacketCounter_Parallel_8(b *testing.B) {
	counter := NewPacketCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
		}
	})
}

// BenchmarkPacketCounter_Parallel_64 并发增加测试 - 64个 goroutine
func BenchmarkPacketCounter_Parallel_64(b *testing.B) {
	counter := NewPacketCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
		}
	})
}

// BenchmarkTransportMetrics_ConcurrentUpdate 测试并发更新传输指标性能
func BenchmarkTransportMetrics_ConcurrentUpdate(b *testing.B) {
	metrics := NewTransportMetrics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RxBytes.Add(100)
		metrics.RxPackets.Add(1)
		metrics.TxBytes.Add(100)
		metrics.TxPackets.Add(1)
	}
}

// BenchmarkTransportMetrics_ParallelUpdate 并发更新传输指标测试 - 16个 goroutine
func BenchmarkTransportMetrics_ParallelUpdate(b *testing.B) {
	metrics := NewTransportMetrics()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.RxBytes.Add(100)
			metrics.RxPackets.Add(1)
			metrics.TxBytes.Add(100)
			metrics.TxPackets.Add(1)
		}
	})
}

// BenchmarkTransportMetrics_UpdateAll 测试批量更新性能
func BenchmarkTransportMetrics_UpdateAll(b *testing.B) {
	metrics := NewTransportMetrics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.UpdateAll()
	}
}

// BenchmarkTransportMetrics_GetAll 测试获取所有指标性能
func BenchmarkTransportMetrics_GetAll(b *testing.B) {
	metrics := NewTransportMetrics()
	metrics.RxBytes.Add(1000)
	metrics.RxPackets.Add(10)
	metrics.TxBytes.Add(1000)
	metrics.TxPackets.Add(10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RxBytes.Get()
		metrics.RxPackets.Get()
		metrics.TxBytes.Get()
		metrics.TxPackets.Get()
	}
}

// BenchmarkTransportMetrics_RealisticScenario 模拟真实场景：高并发写入 + 定期读取更新
func BenchmarkTransportMetrics_RealisticScenario(b *testing.B) {
	metrics := NewTransportMetrics()

	// 模拟 16 个 goroutine 同时写入
	var wg sync.WaitGroup
	writeCount := 0

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 每隔 1000 次操作更新一次统计
		if i%1000 == 0 {
			metrics.UpdateAll()
			continue
		}

		// 模拟并发写入
		writeCount++
		if writeCount%16 == 0 {
			wg.Wait()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.RxBytes.Add(1024)
			metrics.RxPackets.Add(1)
			metrics.TxBytes.Add(1024)
			metrics.TxPackets.Add(1)
		}()
	}

	wg.Wait()
}

// BenchmarkSpeedCalculator_Calculate 测试速度计算器性能
func BenchmarkSpeedCalculator_Calculate(b *testing.B) {
	calc := NewSpeedCalculator()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Calculate(uint64(i * 100))
	}
}

// BenchmarkCounterContention 测试高并发争抢场景性能
func BenchmarkCounterContention(b *testing.B) {
	counter := NewBytesCounter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counter.Add(1)
			counter.Get()
			counter.Add(1)
			counter.Get()
		}
	})
}

// BenchmarkAtomicVsStandard 原子操作 vs 标准变量对比（仅供参考）
func BenchmarkAtomicVsStandard_Atomic(b *testing.B) {
	var counter atomic.Uint64

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Add(1)
		_ = counter.Load()
	}
}

// BenchmarkAtomicVsStandard_Standard 使用标准变量（非并发安全）
func BenchmarkAtomicVsStandard_Standard(b *testing.B) {
	var counter uint64

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter++
		_ = counter
	}
}

// BenchmarkBytesCounter_MixedOperations 混合操作性能测试
func BenchmarkBytesCounter_MixedOperations(b *testing.B) {
	counter := NewBytesCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch i % 4 {
		case 0:
			counter.Add(100)
		case 1:
			counter.Add(50)
		case 2:
			counter.Get()
		case 3:
			counter.UpdateStats()
		}
	}
}

// BenchmarkTransportMetrics_Reset 测试重置操作性能
func BenchmarkTransportMetrics_Reset(b *testing.B) {
	metrics := NewTransportMetrics()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metrics.RxBytes.Add(1000)
		metrics.RxPackets.Add(10)
		metrics.Reset()
	}
}
