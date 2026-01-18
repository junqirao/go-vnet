package hub

import (
	"context"
	"fmt"
	"runtime"

	"github.com/google/uuid"
	tun "github.com/sagernet/sing-tun"

	"go-vnet/vnet/protocol"
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
			err = h.readDeviceLinux()
		default:
		}
		err = h.readDevice()
	}()

	for e := range h.txEventChan {
		for i := 0; i < e.N; i++ {
			v, ok := h.router.Route(e.Buffer[i][h.cfg.HeaderSize : e.Sizes[i]+h.cfg.HeaderSize])
			if !ok || v == nil {
				// drop non network packet or loopback
				continue
			}
			dst := v.(*Destination)
			fmt.Println("dst id", dst.id)
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

func (h *Hub) readDeviceLinux() (err error) {
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

type (
	Destination struct {
		TxAdaptor
		ip          string
		id          string
		ctx         context.Context
		ref         *Hub
		sig         chan struct{}
		tx          protocol.ReadWriter
		txEventChan chan *txEvent
	}
	TxError struct {
		dst *Destination
		Err error
	}
	TxAdaptor interface {
		OnDialRx(ctx context.Context) (rw protocol.ReadWriter, err error)
		OnError(ctx context.Context, e *TxError)
	}
)

func (e *TxError) Error() string {
	return fmt.Errorf("connection %s error caused: %w",
		e.dst.id, e.Err).Error()
}

func NewDestination(ctx context.Context, ip string, a TxAdaptor, ref *Hub) *Destination {
	c := &Destination{
		TxAdaptor:   a,
		ip:          ip,
		id:          uuid.New().String(),
		ctx:         ctx,
		ref:         ref,
		sig:         make(chan struct{}),
		txEventChan: make(chan *txEvent, ref.cfg.MaxTxEventBuf),
	}
	go c.txLoop()
	return c
}

func (c *Destination) PushTxEvent(e *txEvent) (err error) {
	if c.tx == nil {
		if err = c.negotiate(); err != nil {
			c.OnError(c.ctx, &TxError{
				dst: c,
				Err: err,
			})
			return
		}
	}
	c.txEventChan <- e
	return
}

func (c *Destination) negotiate() (err error) {
	if c.tx != nil {
		return
	}
	tx, err := c.OnDialRx(c.ctx)
	if err != nil {
		return
	}
	ups := tx.Upstream()
	c.ref.logger.Infof(c.ctx, "send negotiate packet: %s", c.ip)
	_, err = ups.Write([]byte(c.ip))
	if err != nil {
		return
	}
	buf := make([]byte, 1)
	if _, err = ups.Read(buf); err != nil {
		return
	}
	okOrNot := func(b byte) string {
		if b == 1 {
			return "success"
		}
		return "failed"
	}
	c.ref.logger.Infof(c.ctx, "negotiate response: %s", okOrNot(buf[0]))
	if buf[0] != 1 {
		return fmt.Errorf("negotiate failed: %d", buf[0])
	}
	c.tx = tx
	return
}

func (c *Destination) txLoop() {
	var (
		evs   = make([]*txEvent, c.ref.cfg.MaxTxEventBuf)
		buf   = make([][]byte, c.ref.cfg.MaxTxEventBuf)
		sizes = make([]int, c.ref.cfg.MaxTxEventBuf)
		err   error
	)

	for {
		select {
		case <-c.sig:
			return
		case <-c.ref.sig:
			return
		default:
		}

		length := len(c.txEventChan)
		if length > 1 {
			offset := 0
			nEvent := 0
			for ; nEvent < length && offset <= c.ref.cfg.MaxTxEventBuf-c.ref.cfg.BatchSize; nEvent++ {
				event := <-c.txEventChan
				evs[nEvent] = event
				for i := 0; i < event.N; i++ {
					buf[offset] = (event.Buffer)[i]
					sizes[offset] = (event.Sizes)[i]
					offset++
				}
			}
			_, err = c.tx.BatchWrite(buf[:offset], sizes[:offset], c.ref.cfg.HeaderSize)
			if err != nil {
				c.OnError(c.ctx, &TxError{
					dst: c,
					Err: err,
				})
			}
			for i := 0; i < nEvent; i++ {
				c.ref.putTxEvent(evs[i])
			}
		} else {
			event := <-c.txEventChan
			_, err = c.tx.BatchWrite((event.Buffer)[:event.N], (event.Sizes)[:event.N], c.ref.cfg.HeaderSize)
			if err != nil {
				c.OnError(c.ctx, &TxError{
					dst: c,
					Err: err,
				})
			}
			c.ref.putTxEvent(event)
		}
	}
}

func (c *Destination) Close() error {
	close(c.sig)
	return nil
}
