package pool

import "sync"

// BufferedPool 对象池封装，支持任意类型
// 使用预填充 channel + sync.Pool 混合设计，保证最小可用数量
type BufferedPool[T any] struct {
	pool sync.Pool
	ch   chan T // 预填充缓冲区（并发安全）
}

// NewBufferedPool 创建新的事件池
// minPoolSize: 预填充的最小对象数量（保证 GC 后仍可用）
// newFunc: 创建新对象的函数
func NewBufferedPool[T any](minPoolSize int, newFunc func() T) *BufferedPool[T] {
	p := &BufferedPool[T]{
		pool: sync.Pool{
			New: func() any {
				return newFunc()
			},
		},
		ch: make(chan T, minPoolSize),
	}

	// 预填充 channel，保证最小可用数量
	for i := 0; i < minPoolSize; i++ {
		p.ch <- p.pool.Get().(T)
	}

	return p
}

// Get 从池中获取对象
// 优先从 channel 缓冲区获取（快速路径），如果为空则从 sync.Pool 获取
func (p *BufferedPool[T]) Get() T {
	select {
	case e := <-p.ch:
		// 快速路径：从预填充的 channel 获取
		return e
	default:
		// channel 空了，从 sync.Pool 获取
		return p.pool.Get().(T)
	}
}

// Put 将对象放回池中
// 使用非阻塞 select 尝试放回 channel，如果满则放回 sync.Pool
func (p *BufferedPool[T]) Put(e T) {
	select {
	case p.ch <- e:
		// 成功放回 channel
	default:
		// channel 满了，放回 sync.Pool
		p.pool.Put(e)
	}
}

// Size 返回 channel 缓冲区中的对象数量
// 用于监控和调试
func (p *BufferedPool[T]) Size() int {
	return len(p.ch)
}
