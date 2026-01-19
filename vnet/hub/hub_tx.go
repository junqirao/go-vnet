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
		batchSize = h.cfg.BatchSize
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

	// Pre-allocate slices to avoid allocations in hot path
	type routeInfo struct {
		dst   *Destination
		index int
	}
	var routes []routeInfo

	// Destination batch cache: map dst -> event and dst for final send
	type dstBatch struct {
		dst   *Destination
		event *txEvent
	}
	dstBatches := make(map[string]*dstBatch)

	for e := range h.txEventChan {
		n := e.N
		if len(routes) < n {
			routes = make([]routeInfo, n)
		}

		validRoutes := 0
		// Batch route all packets first
		for i := 0; i < n; i++ {
			v, ok := h.router.Route(e.Buffer[i][h.cfg.HeaderSize : e.Sizes[i]+h.cfg.HeaderSize])
			if !ok || v == nil {
				continue
			}
			routes[validRoutes].dst = v.(*Destination)
			routes[validRoutes].index = i
			validRoutes++
		}

		// Fast path: single destination for all packets
		if validRoutes > 0 {
			firstDst := routes[0].dst
			allSame := true
			for i := 1; i < validRoutes; i++ {
				if routes[i].dst != firstDst {
					allSame = false
					break
				}
			}

			if allSame {
				_ = firstDst.PushTxEvent(e)
				continue
			}
		}

		// Multiple destinations: aggregate and send
		for i := 0; i < validRoutes; i++ {
			dst := routes[i].dst
			batch := dstBatches[dst.id]
			if batch == nil {
				batch = &dstBatch{
					dst:   dst,
					event: h.getTxEvent(),
				}
				dstBatches[dst.id] = batch
			}

			// Copy packet data
			copy(batch.event.Buffer[batch.event.N], e.Buffer[routes[i].index])
			batch.event.Sizes[batch.event.N] = e.Sizes[routes[i].index]
			batch.event.N++

			// Send if batch is full
			if batch.event.N == batchSize {
				_ = dst.PushTxEvent(batch.event)
				delete(dstBatches, dst.id)
			}
		}

		// Recycle source event
		h.putTxEvent(e)

		// Send remaining batches
		for _, batch := range dstBatches {
			if batch.event.N > 0 {
				_ = batch.dst.PushTxEvent(batch.event)
			} else {
				h.putTxEvent(batch.event)
			}
		}
		clear(dstBatches)
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
	// for i := 0; i < e.N; i++ {
	// 	fmt.Printf("tx[%d/%d]->%v\n", i+1, e.N, e.Buffer[i][:e.Sizes[i]])
	// }
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
		batch := min(c.ref.cfg.MaxTxEventBuf, length)
		if length > 1 {
			for i := 0; i < batch; i++ {
				event := <-c.txEventChan
				evs[i] = event
				buf[i] = event.Buffer[0]
				sizes[i] = event.Sizes[0]
			}
		} else {
			event := <-c.txEventChan
			evs[0] = event
			batch = 1
			buf[0] = event.Buffer[0]
			sizes[0] = event.Sizes[0]
		}
		_, err = c.tx.BatchWrite(buf[:batch], sizes[:batch], c.ref.cfg.HeaderSize)
		if err != nil {
			c.OnError(c.ctx, &TxError{
				dst: c,
				Err: err,
			})
		}
		for i := 0; i < batch; i++ {
			c.ref.putTxEvent(evs[i])
		}
	}
}

func (c *Destination) Close() error {
	close(c.sig)
	return nil
}
