package hub

import (
	"go-vnet/vnet/protocol"
)

type (
	rxEvent struct {
		packet *[protocol.MaxTransportByteSize]byte
		buf    [][]byte
		sizes  []int
		n      int
	}
	txEvent struct {
		Buffer [][]byte
		Sizes  []int
		N      int
	}
)

func (h *Hub) getRxEvent() *rxEvent {
	return h.rxEventPool.Get().(*rxEvent)
}

func (h *Hub) putRxEvent(e *rxEvent) {
	h.rxEventPool.Put(e)
}

func (h *Hub) getTxEvent() *txEvent {
	return h.txEventPool.Get().(*txEvent)
}

func (h *Hub) putTxEvent(e *txEvent) {
	h.txEventPool.Put(e)
}
