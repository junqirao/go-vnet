package hub

import (
	"runtime"

	tun "github.com/sagernet/sing-tun"
)

func (h *Hub) txLoop() {
	var (
		err       error
		tmpEvents = make(map[string]*txEvent)
		dstMap    = make(map[string]*Destination)
	)

	defer func() {
		if err != nil {
			h.Stop(err.Error())
		}
	}()

	go func() {
		switch runtime.GOOS {
		case "linux":
			err = h.readDevLinux()
		default:
		}
		err = h.readDevice()
	}()

	for e := range h.txEventChan {
		for i := 0; i < e.N; i++ {
			v, ok := h.router.Route(e.Buffer[i][h.cfg.HeaderSize : e.Sizes[i]+h.cfg.HeaderSize])
			if !ok {
				// drop
				continue
			}
			dst := v.(*Destination)
			event, ok := tmpEvents[dst.id]
			if !ok {
				event = h.getTxEvent()
				tmpEvents[dst.id] = event
				dstMap[dst.id] = dst
			}
			// send if full
			if event.N+1 == h.cfg.BatchSize {
				_ = dst.PushTxEvent(event)
				event = h.getTxEvent()
				tmpEvents[dst.id] = event
			}
			copy(event.Buffer[event.N], e.Buffer[i])
			event.Sizes[event.N] = e.Sizes[i]
			event.N++
		}
		for id, event := range tmpEvents {
			if event.N <= 0 {
				h.putTxEvent(event)
				continue
			}
			dst, ok := dstMap[id]
			if !ok || dst == nil {
				h.putTxEvent(event)
				continue
			}
			_ = dst.PushTxEvent(event)
			h.putTxEvent(event)
		}
		clear(tmpEvents)
	}
}

func (h *Hub) readDevice() (err error) {
	h.Infof("read device loop started")
	var (
		n int
	)

	for {
		select {
		case <-h.sig:
			return
		default:
		}
		e := h.getTxEvent()
		n, err = h.dev.Read(e.Buffer[0])
		if err != nil {
			h.putTxEvent(e)
			return
		}
		e.Sizes[0] = n
		e.N = 1
		if n == 0 {
			h.putTxEvent(e)
			continue
		}
		h.txEventChan <- e
	}
}

func (h *Hub) readDevLinux() (err error) {
	var (
		n      int
		dev    = h.dev.(tun.LinuxTUN)
		offset = dev.FrontHeadroom()
	)

	h.Infof("batch read device loop started, batch size: %d, header size: %d",
		dev.BatchSize(), offset)
	for {
		select {
		case <-h.sig:
			return
		default:
		}
		e := h.getTxEvent()
		n, err = dev.BatchRead(e.Buffer, offset, e.Sizes)
		if err != nil {
			h.putTxEvent(e)
			return
		}
		e.N = n
		if n == 0 {
			h.putTxEvent(e)
			continue
		}
		h.txEventChan <- e
	}
}
