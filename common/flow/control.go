package flow

import (
	"context"
	"sync"
)

type (
	// Handler 是中间件处理器函数
	// 参数说明：
	//   - sess: 会话对象
	//   - data: 输入数据
	//   - buf: 预分配的可复用缓冲区，容量可通过 cap(buf) 获取
	//
	// 返回值说明：
	//   - 如果需要原地修改，直接返回 data（零分配）
	//   - 如果需要构造新数据，使用 buf[:] 并返回（复用内存）
	//   - buf 使用方式：buf = buf[:0]; buf.Write(...); return buf
	//
	// 示例 - 原地修改（零分配）：
	//   func handler(sess *session.Session, data []byte, buf []byte) []byte {
	//       data[0] ^= 0xFF
	//       return data
	//   }
	//
	// 示例 - 添加数据（复用 buf）：
	//   func handler(sess *session.Session, data []byte, buf []byte) []byte {
	//       buf = buf[:len(data)+4]
	//       copy(buf[4:], data)
	//       copy(buf[:4], []byte{0, 0, 0, 1})
	//       return buf
	//   }
	Handler func(ctx context.Context, data []byte, buf []byte) []byte

	Control struct {
		handlers   []Handler
		bufferPool *sync.Pool
	}
)

var (
	NopControl = &Control{}
)

var defaultBufferPool = &sync.Pool{
	New: func() interface{} {
		return make([]byte, 0, 2048)
	},
}

// NewControl 创建一个新的控制链
func NewControl(handlers ...Handler) *Control {
	return &Control{
		handlers:   handlers,
		bufferPool: defaultBufferPool,
	}
}

// Handle 执行中间件链，自动管理内存池
// - 自动复用缓冲区，最小化内存分配
// - 支持原地修改（零分配）和构造新数据（复用 buf）两种模式
func (c *Control) Handle(ctx context.Context, data []byte) []byte {
	if len(c.handlers) == 0 {
		return data
	}

	input := data
	buf := c.bufferPool.Get().([]byte)
	defer c.bufferPool.Put(buf)

	for _, h := range c.handlers {
		// 重置 buf，复用内存
		buf = buf[:0]

		// 动态调整 buf 容量，确保足够大
		requiredCap := len(input) + 256
		if cap(buf) < requiredCap {
			// 按需扩容
			buf = make([]byte, 0, requiredCap)
		}

		// 执行中间件
		input = h(ctx, input, buf)
	}

	return input
}

// AddHandler 添加中间件到链的末尾
func (c *Control) AddHandler(handler Handler) {
	c.handlers = append(c.handlers, handler)
}

// Clear 清空所有中间件
func (c *Control) Clear() {
	c.handlers = c.handlers[:0]
}

// Len 返回中间件数量
func (c *Control) Len() int {
	return len(c.handlers)
}
