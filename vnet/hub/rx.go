package hub

import (
	"errors"
	"io"
	"net"

	"go-vnet/vnet/protocol"
)

func (h *Hub) HandleRx(rwc io.ReadWriteCloser) {
	type remotable interface {
		RemoteAddr() net.Addr
	}

	go func() {
		remote := ""
		if r, ok := rwc.(remotable); ok {
			remote = r.RemoteAddr().String()
		}

		defer func() {
			h.Infof("rx from closed: %s", remote)
			_ = rwc.Close()
		}()
		h.Infof("rx from started %s", remote)

		rw := protocol.NewTransport(rwc)
		for {
			event := h.getRxEvent()
			typ, n, err := rw.ReadMessage((*event.packet)[:])
			if errors.Is(err, protocol.ErrInvalidMagic) {
				h.putRxEvent(event)
				continue
			}
			if err != nil {
				h.putRxEvent(event)
				return
			}
			switch typ {
			case protocol.TypeTransport:
				event.n = 1
				event.sizes[0] = n
				event.buf[0] = (*event.packet)[:n]
				h.rxEventChan <- event
			case protocol.TypeBatchTransport:
				_, err = rw.ParseBatch((*event.packet)[:n], event.buf, event.sizes, h.cfg.HeaderSize)
				if err != nil {
					h.putRxEvent(event)
					return
				}
				event.n = n
				h.rxEventChan <- event
			}
		}
	}()
}
