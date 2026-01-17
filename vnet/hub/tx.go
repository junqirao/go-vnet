package hub

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"go-vnet/vnet/protocol"
)

type (
	Destination struct {
		TxAdaptor
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

func NewDestination(ctx context.Context, a TxAdaptor, ref *Hub) *Destination {
	c := &Destination{
		TxAdaptor:   a,
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
		c.tx, err = c.OnDialRx(c.ctx)
	}
	c.txEventChan <- e
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
