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
	return h.rxEventPool.Get()
}

func (h *Hub) putRxEvent(e *rxEvent) {
	e.n = 0
	h.rxEventPool.Put(e)
}

func (h *Hub) getTxEvent() *txEvent {
	return h.txEventPool.Get()
}

func (h *Hub) putTxEvent(e *txEvent) {
	e.N = 0
	h.txEventPool.Put(e)
}
