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
	// 使用非阻塞 select 优先从 channel 缓冲区获取
	// select 有两个 case：1) 尝试从 rxEventCh 获取（非阻塞），2) 从 sync.Pool 获取
	select {
	case e := <-h.rxEventCh:
		// 快速路径：从预填充的 channel 获取（无锁或低锁）
		return e
	default:
		// channel 空了，从 sync.Pool 获取
		return h.rxEventPool.Get().(*rxEvent)
	}
}

func (h *Hub) putRxEvent(e *rxEvent) {
	e.n = 0
	// 使用非阻塞 select 尝试放回 channel
	// 如果 channel 满（说明已经有足够的预填充对象），放回 sync.Pool
	select {
	case h.rxEventCh <- e:
		// 成功放回 channel（无阻塞）
	default:
		// channel 满了，放回 sync.Pool
		h.rxEventPool.Put(e)
	}
}

func (h *Hub) getTxEvent() *txEvent {
	// 使用非阻塞 select 优先从 channel 缓冲区获取
	select {
	case e := <-h.txEventCh:
		// 快速路径：从预填充的 channel 获取
		return e
	default:
		// channel 空了，从 sync.Pool 获取
		return h.txEventPool.Get().(*txEvent)
	}
}

func (h *Hub) putTxEvent(e *txEvent) {
	e.N = 0
	// 使用非阻塞 select 尝试放回 channel
	select {
	case h.txEventCh <- e:
		// 成功放回 channel
	default:
		// channel 满了，放回 sync.Pool
		h.txEventPool.Put(e)
	}
}
