package hub

import (
	"go-vnet/common/logger"
)

func (h *Hub) SetLogger(l logger.Logger) {
	h.logger = l
}

func (h *Hub) Infof(format string, args ...any) {
	h.logger.Infof(h.ctx, format, args...)
}

func (h *Hub) Errorf(format string, args ...any) {
	h.logger.Errorf(h.ctx, format, args...)
}
