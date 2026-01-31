package hub

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"

	"github.com/google/uuid"
	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/protocol"
)

var (
	ErrCancel = errors.New("cancel")
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
	var (
		routes       = make([]routeInfo, h.cfg.BatchSize)
		packetSlices = make([][]byte, h.cfg.BatchSize)
		targets      = make([]any, h.cfg.BatchSize)
		e            *txEvent
	)

	// Destination batch cache: map dst -> event and dst for final send
	type dstBatch struct {
		dst   *Destination
		event *txEvent
	}
	dstBatches := make(map[string]*dstBatch)

	for {
		select {
		case <-h.sig:
			return
		case e = <-h.txEventChan:
		}
		n := e.N
		validRoutes := 0
		// 准备批量路由的数据包切片（避免循环内重复计算偏移）
		for i := 0; i < n; i++ {
			packetSlices[i] = e.Buffer[i][h.cfg.HeaderSize : e.Sizes[i]+h.cfg.HeaderSize]
		}

		// 批量路由查询，减少循环开销
		validRouteCount := h.router.RouteBatch(packetSlices[:n], targets[:n])

		// 收集有效路由（单次遍历）
		for i := 0; i < n && validRoutes < validRouteCount; i++ {
			if targets[i] != nil {
				routes[validRoutes].dst = targets[i].(*Destination)
				routes[validRoutes].index = i
				validRoutes++
			}
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

			// Copy packet data（直接索引访问，减少变量查找）
			srcIdx := routes[i].index
			dstIdx := batch.event.N
			copy(batch.event.Buffer[dstIdx], e.Buffer[srcIdx])
			batch.event.Sizes[dstIdx] = e.Sizes[srcIdx]
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
		if n == 0 {
			h.putTxEvent(e)
			continue
		}
		e.N = n
		h.txEventChan <- e
	}
}

type (
	Destination struct {
		TxAdaptor
		mu          sync.RWMutex
		ip          string
		id          string
		ctx         context.Context
		ref         *Hub
		sig         chan struct{}
		tx          protocol.ReadWriter
		txEventChan chan *txEvent
		fallback    protocol.ReadWriter
		hook        TxHook
	}
	TxError struct {
		dst *Destination
		Err error
	}
	TxAdaptor interface {
		Dial(ctx context.Context, dst string) (rw protocol.ReadWriter, err error)
		CloseDst(ctx context.Context, dst string)
		OnError(ctx context.Context, e *TxError)
	}
	TxHook interface {
		AfterDial(ctx context.Context, dst *Destination)
		OnFallback(ctx context.Context, dst *Destination, rw protocol.ReadWriter, err error)
	}
	nopTxHook struct{}
)

func (n nopTxHook) AfterDial(ctx context.Context, dst *Destination) {}

func (n nopTxHook) OnFallback(ctx context.Context, dst *Destination, rw protocol.ReadWriter, err error) {
}

func (e *TxError) Error() string {
	return fmt.Errorf("connection %s error caused: %w",
		e.dst.id, e.Err).Error()
}

func (e *TxError) Dst() *Destination {
	return e.dst
}

func NewDestination(ctx context.Context, ip string, a TxAdaptor, ref *Hub, h ...TxHook) *Destination {
	var hook TxHook = &nopTxHook{}
	if len(h) > 0 {
		hook = h[0]
	}
	c := &Destination{
		TxAdaptor:   a,
		ip:          ip,
		id:          uuid.New().String(),
		ctx:         ctx,
		ref:         ref,
		sig:         make(chan struct{}),
		txEventChan: make(chan *txEvent, ref.cfg.MaxTxEventBuf),
		hook:        hook,
	}
	if ref.cfg.BatchSize > 1 {
		go c.txLoopN()
	} else {
		go c.txLoop()
	}
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
	tx, err := c.Dial(c.ctx, c.ip)
	if err != nil {
		return
	}
	ups := tx.Upstream()
	c.ref.logger.Infof(c.ctx, "[TX] send negotiate packet: %s", c.ip)
	_, err = ups.Write([]byte(c.ip))
	if err != nil {
		return
	}
	var buf [1]byte
	if _, err = ups.Read(buf[:]); err != nil {
		return
	}
	if buf[0] != 1 {
		c.ref.logger.Infof(c.ctx, "[TX] negotiate failed: %d", buf[0])
		return fmt.Errorf("negotiate failed: %d", buf[0])
	}
	c.ref.logger.Infof(c.ctx, "[TX] negotiate success")
	c.tx = tx
	c.hook.AfterDial(c.ctx, c)
	return
}

func (c *Destination) txLoop() {
	var (
		evs   = make([]*txEvent, c.ref.cfg.MaxTxEventBuf)
		buf   = make([][]byte, c.ref.cfg.MaxTxEventBuf)
		sizes = make([]int, c.ref.cfg.MaxTxEventBuf)
		err   error
	)

	defer func() {
		c.ref.logger.Infof(c.ctx, "[TX] txLoop exit: dst=%s,err=%v", c.ip, err)
	}()
	c.ref.logger.Infof(c.ctx, "[TX] txLoop start: dst=%s", c.ip)

	for {
		select {
		case <-c.sig:
			return
		case <-c.ref.sig:
			return
		case event := <-c.txEventChan:
			evs[0] = event
			buf[0] = event.Buffer[0]
			sizes[0] = event.Sizes[0]
			batch := 1

			// Batch collect more events without blocking
			for batch < c.ref.cfg.MaxTxEventBuf && len(c.txEventChan) > 0 {
				event = <-c.txEventChan
				evs[batch] = event
				buf[batch] = event.Buffer[0]
				sizes[batch] = event.Sizes[0]
				batch++
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
}

func (c *Destination) txLoopN() {
	var (
		buf            = make([][]byte, c.ref.cfg.MaxTxEventBuf)
		sizes          = make([]int, c.ref.cfg.MaxTxEventBuf)
		eventsToReturn = make([]*txEvent, c.ref.cfg.MaxTxEventBuf)
		err            error
	)
	maxBuf := c.ref.cfg.MaxTxEventBuf

	defer func() {
		c.ref.logger.Infof(c.ctx, "[TX] txLoopN exit: dst=%s,err=%v", c.ip, err)
	}()
	c.ref.logger.Infof(c.ctx, "[TX] txLoopN start: dst=%s", c.ip)

	for {
		select {
		case <-c.sig:
			return
		case <-c.ref.sig:
			return
		case event := <-c.txEventChan:
			totalPackets := 0
			eventCount := 0
			remainingBuf := maxBuf

			// Expand first event's packets
			for j := 0; j < event.N; j++ {
				buf[totalPackets] = event.Buffer[j]
				sizes[totalPackets] = event.Sizes[j]
				totalPackets++
				remainingBuf--
			}
			eventsToReturn[eventCount] = event
			eventCount++

			// Batch collect more events without blocking
			for remainingBuf > 0 {
				if len(c.txEventChan) == 0 {
					break
				}
				event = <-c.txEventChan
				eventsToReturn[eventCount] = event
				eventCount++

				// Pack packets from this event
				for j := 0; j < event.N && remainingBuf > 0; j++ {
					buf[totalPackets] = event.Buffer[j]
					sizes[totalPackets] = event.Sizes[j]
					totalPackets++
					remainingBuf--
				}
			}

			// Write all collected packets
			if totalPackets > 0 {
				_, err = c.tx.BatchWrite(buf[:totalPackets], sizes[:totalPackets], c.ref.cfg.HeaderSize)
				if err != nil {
					c.fallbackOrReportError(err)
				}
			}

			// Return all events to pool（批量释放，减少锁竞争）
			for i := 0; i < eventCount; i++ {
				c.ref.putTxEvent(eventsToReturn[i])
			}
		}
	}
}

func (c *Destination) Ip() string {
	return c.ip
}

func (c *Destination) Close() error {
	close(c.sig)
	if c.fallback != nil {
		_ = c.fallback.Close()

	}
	if c.tx != nil {
		return c.tx.Close()
	}
	return nil
}

func (c *Destination) ReplaceTx(fn func(old protocol.ReadWriter) (new protocol.ReadWriter, replaced bool)) (replaced bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// use c.tx as fall back
	if c.fallback != nil {
		return
	}
	newOne, replaced := fn(c.tx)
	if replaced {
		c.fallback = c.tx
		c.tx = newOne
	}
	return
}

func (c *Destination) fallbackOrReportError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	curr := c.tx
	if c.fallback != nil {
		c.tx = c.fallback
		c.hook.OnFallback(c.ctx, c, curr, err)
		return
	}
	c.OnError(c.ctx, &TxError{
		dst: c,
		Err: err,
	})
}

func (c *Destination) Type() string {
	return c.tx.Type()
}
