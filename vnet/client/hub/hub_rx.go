package hub

import (
	"errors"
	"io"
	"runtime"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/protocol"
)

type (
	RxHook interface {
		OnStart()
		OnClose(err error)
	}
	noopRxHook struct{}
)

func (n noopRxHook) OnStart() {}

func (n noopRxHook) OnClose(err error) {}

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
		// fmt.Printf("rx->%v\n", event.buf[:event.n])
		_, err := dev.BatchWrite(event.buf[:event.n], h.cfg.HeaderSize)
		if err != nil {
			h.Stop(err.Error())
			return
		}
		// Return event to pool for reuse
		h.putRxEvent(event)
	}
}

func (h *Hub) writeDevice() {
	h.Infof("write device loop started")
	for event := range h.rxEventChan {
		for i := 0; i < event.n; i++ {
			// fmt.Printf("rx->%v\n", event.buf[i][:event.sizes[i]])
			_, err := h.dev.Write(event.buf[i][:event.sizes[i]])
			if err != nil {
				h.Stop(err.Error())
				return
			}
		}
		// Return event to pool for reuse
		h.putRxEvent(event)
	}
}

func (h *Hub) HandleRx(rwc io.ReadWriteCloser, hook ...RxHook) (cancel func()) {
	var (
		sig = make(chan struct{})
	)

	cancel = func() {
		select {
		case _, ok := <-sig:
			if !ok {
				return
			}
		default:
		}
		close(sig)
	}

	go func() {
		var (
			rw     = protocol.NewTransport(rwc)
			offset = h.cfg.HeaderSize
			nn     int
			err    error
			typ    byte
			n      int
			ho     RxHook
		)
		if len(hook) > 0 {
			ho = hook[0]
		} else {
			ho = &noopRxHook{}
		}

		ho.OnStart()
		defer func() {
			_ = rwc.Close()
			cancel()
			ho.OnClose(err)
		}()

		for {
			select {
			case <-h.sig:
				return
			case <-sig:
				return
			default:
			}

			event := h.getRxEvent()
			typ, n, err = rw.ReadMessage((*event.packet)[:])
			// fmt.Printf("read message: type=%d n=%d, err=%v\n", typ, n, err)
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
				copy(event.buf[0][offset:offset+n], (*event.packet)[:n])
				h.rxEventChan <- event
			case protocol.TypeBatchTransport:
				nn, err = rw.ParseBatch((*event.packet)[:n], event.buf, event.sizes, offset)
				if err != nil {
					h.putRxEvent(event)
					return
				}
				event.n = nn
				h.rxEventChan <- event
			}
		}
	}()
	return
}
