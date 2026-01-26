package flow

import (
	"context"
)

type (
	Handler func(ctx context.Context, data []byte)

	Control struct {
		handlers []Handler
	}
)

var (
	NopControl = &Control{}
)

// NewControl 创建一个新的控制链
func NewControl(handlers ...Handler) *Control {
	return &Control{
		handlers: handlers,
	}
}

// Handle 执行中间件链，自动管理内存池
// - 自动复用缓冲区，最小化内存分配
// - 支持原地修改（零分配）和构造新数据（复用 buf）两种模式
func (c *Control) Handle(ctx context.Context, data []byte) []byte {
	if len(c.handlers) == 0 {
		return data
	}

	for _, h := range c.handlers {
		// 重置 buf，复用内存
		// 执行中间件
		h(ctx, data)
	}

	return data
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
