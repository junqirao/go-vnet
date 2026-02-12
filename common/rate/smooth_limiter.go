package rate

import (
	"sync/atomic"
	"time"
)

// SmoothLimiter 平滑限速器，基于时间控制实现恒定速率传输
// 核心思想：每次传输后根据字节数计算期望的传输时间，确保平均速率恒定
type SmoothLimiter struct {
	rate      int64   // 速率（字节/秒）
	lastTime  int64   // 上次传输时间（纳秒）
	smoothing float64 // 平滑因子，用于缓冲小包传输
}

// NewSmoothLimiter 创建平滑限速器
// rate: 速率（字节/秒）
func NewSmoothLimiter(rate int) *SmoothLimiter {
	return &SmoothLimiter{
		rate:      int64(rate),
		lastTime:  time.Now().UnixNano(),
		smoothing: 1.05, // 平滑因子：1.05 补偿 Sleep 系统开销，达到精确限速
	}
}

// Wait 等待直到可以传输 1 个字节
func (l *SmoothLimiter) Wait() {
	l.WaitN(1)
}

// WaitN 等待直到可以传输 n 个字节
// 核心算法：
// 1. 计算传输 n 字节期望的时间间隔 = n / rate
// 2. 计算自上次传输后的实际时间间隔
// 3. 如果实际间隔 < 期望间隔，Sleep 补足差额
// 4. 更新最后传输时间
func (l *SmoothLimiter) WaitN(n int) {
	if n <= 0 || l.rate <= 0 {
		return
	}

	// 计算期望的最小间隔时间（纳秒）
	// smoothing = 0.95 稍微放宽限制，补偿 Sleep 的系统开销
	expectedInterval := int64(float64(n*1e9) / float64(l.rate) / l.smoothing)

	now := time.Now().UnixNano()

	// 原子读取上次传输时间
	last := atomic.LoadInt64(&l.lastTime)

	// 计算自上次传输后的实际时间间隔
	elapsed := now - last

	// 如果实际间隔小于期望间隔，需要等待
	if elapsed < expectedInterval {
		waitTime := expectedInterval - elapsed
		if waitTime > 0 {
			time.Sleep(time.Duration(waitTime))
		}
	}

	// 原子更新最后传输时间
	atomic.StoreInt64(&l.lastTime, time.Now().UnixNano())
}

// SetSmoothing 设置平滑因子
// factor: 平滑因子，范围 (0, 1]
//   - 1.0: 完全平滑，精确限速
//   - 0.9: 允许10%的突发，减少小包阻塞
//   - 0.5: 允许50%的突发
func (l *SmoothLimiter) SetSmoothing(factor float64) {
	if factor > 0 && factor <= 1.0 {
		l.smoothing = factor
	}
}

// Rate 返回当前速率（字节/秒）
func (l *SmoothLimiter) Rate() int {
	return int(atomic.LoadInt64(&l.rate))
}

// SetRate 设置新的速率（字节/秒）
func (l *SmoothLimiter) SetRate(rate int) {
	atomic.StoreInt64(&l.rate, int64(rate))
}
