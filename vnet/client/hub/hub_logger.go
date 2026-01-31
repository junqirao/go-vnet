package hub

import (
	"context"

	"go-vnet/common/logger"
)

// 预定义的空 logger，用于避免条件判断的开销
type noopLogger struct{}

func (l *noopLogger) Info(ctx context.Context, msg string)                   {}
func (l *noopLogger) Error(ctx context.Context, msg string)                  {}
func (l *noopLogger) Infof(ctx context.Context, format string, args ...any)  {}
func (l *noopLogger) Errorf(ctx context.Context, format string, args ...any) {}

var noopLoggerInstance logger.Logger = &noopLogger{}

func (h *Hub) SetLogger(l logger.Logger) {
	if l == nil {
		h.logger = noopLoggerInstance
	} else {
		h.logger = l
	}
}

func (h *Hub) Infof(format string, args ...any) {
	h.logger.Infof(h.ctx, format, args...)
}

func (h *Hub) Errorf(format string, args ...any) {
	h.logger.Errorf(h.ctx, format, args...)
}
