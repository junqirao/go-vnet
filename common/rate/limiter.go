package rate

import (
	"sync/atomic"
	"time"
)

// Limiter 无锁限流器，基于令牌桶算法
// 使用原子操作实现，无需互斥锁，适合高并发场景
type Limiter struct {
	lastUpdate int64 // 上次更新时间（纳秒）
	available  int64 // 可用令牌数（固定精度，乘以precision）
	rate       int64 // 速率（令牌/秒，乘以precision）
	burst      int64 // 桶容量（令牌数，乘以precision）
	precision  int64 // 精度因子，用于避免浮点运算
}

// NewLimiter 创建无锁限流器
// rate: 每秒允许的请求数
// burst: 桶容量（最大突发请求数）
func NewLimiter(rate, burst int) *Limiter {
	precision := int64(1000000) // 100万倍精度
	l := &Limiter{
		lastUpdate: time.Now().UnixNano(),
		available:  int64(burst) * precision,
		rate:       int64(rate) * precision,
		burst:      int64(burst) * precision,
		precision:  precision,
	}
	return l
}

// Allow 检查是否允许通过（消费1个令牌）
func (l *Limiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 检查是否允许通过n个令牌
func (l *Limiter) AllowN(n int) bool {
	if n <= 0 {
		return false
	}
	requestTokens := int64(n) * l.precision
	now := time.Now().UnixNano()

	for {
		last := atomic.LoadInt64(&l.lastUpdate)
		elapsed := now - last

		// 计算这段时间内生成的令牌数
		var added int64
		if elapsed > 0 {
			added = elapsed * l.rate / 1e9
		}

		// 计算添加令牌后的预期值
		current := atomic.LoadInt64(&l.available)
		newAvailable := current + added
		if newAvailable > l.burst {
			newAvailable = l.burst
		}

		// 尝试同时更新时间和令牌（使用 CAS）
		// 先尝试更新时间
		if elapsed > 0 && atomic.CompareAndSwapInt64(&l.lastUpdate, last, now) {
			// 如果有添加的令牌，尝试更新可用令牌
			if added > 0 {
				for {
					current := atomic.LoadInt64(&l.available)
					newCurrent := current + added
					if newCurrent > l.burst {
						newCurrent = l.burst
					}
					if atomic.CompareAndSwapInt64(&l.available, current, newCurrent) {
						break
					}
				}
			}
			// 重新读取更新后的可用令牌
			current = atomic.LoadInt64(&l.available)
		}

		// 检查是否有足够的令牌
		current = atomic.LoadInt64(&l.available)
		if current >= requestTokens {
			// 尝试消费令牌
			newCurrent := current - requestTokens
			if atomic.CompareAndSwapInt64(&l.available, current, newCurrent) {
				return true
			}
			// CAS失败，重试
			continue
		}

		// 令牌不足，返回false
		return false
	}
}

// Wait 等待直到可以获取1个令牌
func (l *Limiter) Wait() {
	l.WaitN(1)
}

// WaitN 等待直到可以获取n个令牌
func (l *Limiter) WaitN(n int) {
	for !l.AllowN(n) {
		// 计算需要生成n个令牌的时间
		requestTokens := int64(n) * l.precision
		tokensToGenerate := requestTokens - atomic.LoadInt64(&l.available)
		if tokensToGenerate > 0 {
			waitTime := tokensToGenerate * 1e9 / l.rate
			if waitTime > 0 {
				time.Sleep(time.Duration(waitTime))
			}
		} else {
			time.Sleep(time.Microsecond)
		}
	}
}

// Tokens 返回当前可用令牌数
func (l *Limiter) Tokens() float64 {
	now := time.Now().UnixNano()
	last := atomic.LoadInt64(&l.lastUpdate)
	elapsed := now - last

	current := atomic.LoadInt64(&l.available)

	// 考虑时间流逝
	if elapsed > 0 {
		addTokens := elapsed * l.rate / 1e9
		current += addTokens
		if current > l.burst {
			current = l.burst
		}
	}

	return float64(current) / float64(l.precision)
}
