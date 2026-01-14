package protocol

import (
	"errors"
	"runtime"
	"sync"
)

const (
	eventMaxBufferSize = 65535
)

func defaultPacketEventProcessorOptions() *PacketEventProcessorOptions {
	o := &PacketEventProcessorOptions{
		batchSize:           128, // dev read batch size
		offset:              10,  // gso header
		maxPacketSize:       1399,
		maxBufferedRxEvents: 1024, // rx channel buffer size
		maxBufferedTxEvents: 1024, // tx channel buffer size
	}
	if runtime.GOOS == "windows" {
		o.batchSize = 1
		o.offset = 0
	}
	return o
}

type (
	PacketEventProcessor struct {
		PacketEventProcessorOptions
		rw         ReadWriter
		rEventPool sync.Pool
		rChan      chan *ReadEvent
		wEventPool sync.Pool
		wChan      chan *WriteEvent
	}
	PacketEventProcessorOptions struct {
		batchSize           int
		maxPacketSize       int
		offset              int
		maxBufferedRxEvents int
		maxBufferedTxEvents int
	}
	Opt       func(o *PacketEventProcessorOptions)
	ReadEvent struct {
		buffer *[eventMaxBufferSize]byte
		n      uint16 // 使用 uint16 而不是 int，节省内存（与 protocol 长度字段类型一致）
		typ    byte
	}
	WriteEvent struct {
		Buffer *[][]byte
		Sizes  *[]int
		N      int
	}
)

func WithBatchSize(bs int) Opt {
	return func(o *PacketEventProcessorOptions) {
		o.batchSize = bs
	}
}
func WithBatchOffset(ofs int) Opt {
	return func(o *PacketEventProcessorOptions) {
		o.offset = ofs
	}
}

func newReadEvent() *ReadEvent {
	return &ReadEvent{
		buffer: new([eventMaxBufferSize]byte),
	}
}

func (e *ReadEvent) Type() byte {
	return e.typ
}

func (e *ReadEvent) Bytes() []byte {
	// 使用全切片表达式避免分配新的切片头
	return e.buffer[:e.n:e.n]
}

func (e *ReadEvent) N() int {
	return int(e.n)
}

func newWriteEvent(batchSize, maxPacketSize int) *WriteEvent {
	buf := make([][]byte, batchSize)
	for i := 0; i < batchSize; i++ {
		buf[i] = make([]byte, maxPacketSize)
	}
	sizes := make([]int, batchSize)
	w := &WriteEvent{
		Buffer: &buf,
		Sizes:  &sizes,
		N:      0,
	}
	return w
}

func NewPacketEventProcessor(rw ReadWriter, opts ...Opt) *PacketEventProcessor {
	o := defaultPacketEventProcessorOptions()
	for _, opt := range opts {
		opt(o)
	}
	p := &PacketEventProcessor{
		PacketEventProcessorOptions: *o,
		rw:                          rw,
		rEventPool: sync.Pool{
			New: func() interface{} {
				return newReadEvent()
			},
		},
		wEventPool: sync.Pool{
			New: func() interface{} {
				return newWriteEvent(o.batchSize, o.maxPacketSize)
			},
		},
		rChan: make(chan *ReadEvent, o.maxBufferedRxEvents),
		wChan: make(chan *WriteEvent, o.maxBufferedTxEvents),
	}
	go p.readLoop()
	return p
}

func (p *PacketEventProcessor) RX() <-chan *ReadEvent {
	return p.rChan
}

func (p *PacketEventProcessor) readLoop() {
	defer close(p.rChan)
	for {
		event := p.getRxEvent()
		// 直接传递数组指针，避免创建切片
		typ, n, err := p.rw.ReadMessage((*event.buffer)[:])
		if errors.Is(err, ErrInvalidMagic) {
			p.PutRXEvent(event)
			continue
		}
		if err != nil {
			p.PutRXEvent(event)
			return
		}
		event.n = uint16(n)
		event.typ = typ
		p.rChan <- event
	}
}

func (p *PacketEventProcessor) getRxEvent() *ReadEvent {
	return p.rEventPool.Get().(*ReadEvent)
}

// PutRXEvent back to sync.Pool without clean buf
func (p *PacketEventProcessor) PutRXEvent(e *ReadEvent) {
	if e == nil {
		return
	}
	p.rEventPool.Put(e)
}

// PutTXEvent back to sync.Pool without clean buf
func (p *PacketEventProcessor) PutTXEvent(e *WriteEvent) {
	if e != nil {
		return
	}
	p.wEventPool.Put(e)
}

// GetTXEvent with buffer
func (p *PacketEventProcessor) GetTXEvent() (e *WriteEvent) {
	e = p.wEventPool.Get().(*WriteEvent)
	return
}

// Stop stops the packet event processor
func (p *PacketEventProcessor) Stop() {
	// Note: The readLoop will be stopped when the underlying reader returns an error
	// or when the channel is closed externally
}
