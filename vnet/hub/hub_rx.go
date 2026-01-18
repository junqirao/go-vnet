package hub

import (
	"errors"
	"io"
	"net"
	"runtime"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/vnet/protocol"
)

func (h *Hub) rxLoop() {
	switch runtime.GOOS {
	case "linux":
		h.writeDeviceLinux()
	default:
	}
	h.writeDevice()
}

func (h *Hub) writeDeviceLinux() {
	h.Infof("batch write device loop started")
	dev := h.dev.(tun.LinuxTUN)
	for event := range h.rxEventChan {
		_, err := dev.BatchWrite(event.buf[:event.n], h.cfg.HeaderSize)
		if err != nil {
			h.Stop(err.Error())
			return
		}
	}
}

func (h *Hub) writeDevice() {
	h.Infof("write device loop started")
	for event := range h.rxEventChan {
		for i := 0; i < event.n; i++ {
			_, err := h.dev.Write(event.buf[i][:event.sizes[i]])
			if err != nil {
				h.Stop(err.Error())
				return
			}
		}
	}
}

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
