package protocol

import (
	"errors"
	"runtime"
	"sync"
)

const (
	eventMaxBufferSize = 65535
)

const (
	ErrModuleTx = "tx"
	ErrModuleRx = "rx"
)

func defaultPacketEventProcessorOptions() *PacketEventProcessorOptions {
	o := &PacketEventProcessorOptions{
		batchSize:           128, // dev read batch size
		headerSize:          10,  // gso header
		maxPacketSize:       1399,
		maxBufferedRxEvents: 1024, // rx channel buffer size
		maxBufferedTxEvents: 1024, // tx channel buffer size
	}
	if runtime.GOOS == "windows" {
		o.batchSize = 1
		o.headerSize = 0
	}
	return o
}

type (
	PacketEventProcessor struct {
		PacketEventProcessorOptions
		rw         ReadWriter
		rEventPool sync.Pool        // read(rx) event pool
		rChan      chan *ReadEvent  // read(rx) channel
		wEventPool sync.Pool        // write(tx) event pool
		wChan      chan *WriteEvent // write(tx) channel
		wbChan     chan *WriteEvent // write(tx) batch channel
		errHandler func(module string, err error)
	}
	PacketEventProcessorOptions struct {
		batchSize           int
		maxPacketSize       int
		headerSize          int
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

func WithBatchSize(size int) Opt {
	return func(o *PacketEventProcessorOptions) {
		o.batchSize = size
	}
}
func WithHeaderSize(size int) Opt {
	return func(o *PacketEventProcessorOptions) {
		o.headerSize = size
	}
}

func WithMaxPacketSize(size int) Opt {
	return func(o *PacketEventProcessorOptions) {
		o.maxPacketSize = size
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

func (w *WriteEvent) DeleteElements(i ...int) {
	if len(i) == 0 || len(i) >= len(*w.Sizes) {
		return
	}

	// 双指针：writeIdx 写入位置，skipIdx 跳过索引位置
	skipIdx := 0
	writeIdx := 0
	totalLen := len(*w.Sizes)

	for readIdx := 0; readIdx < totalLen; readIdx++ {
		// 检查当前索引是否需要跳过
		if skipIdx < len(i) && readIdx == i[skipIdx] {
			skipIdx++
			continue
		}
		// 如果不是当前位置才移动
		if readIdx != writeIdx {
			(*w.Buffer)[writeIdx] = (*w.Buffer)[readIdx]
			(*w.Sizes)[writeIdx] = (*w.Sizes)[readIdx]
		}
		writeIdx++
		// 更新有效长度
		w.N--
	}
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
		rChan:  make(chan *ReadEvent, o.maxBufferedRxEvents),
		wChan:  make(chan *WriteEvent, o.maxBufferedTxEvents),
		wbChan: make(chan *WriteEvent, o.maxBufferedTxEvents),
	}
	go p.readLoop()
	go p.writeLoop()
	go p.writeBatchLoop()
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
			if err != nil && p.errHandler != nil {
				p.errHandler(ErrModuleRx, err)
			}
			p.PutRXEvent(event)
			return
		}
		event.n = uint16(n)
		event.typ = typ
		p.rChan <- event
	}
}

func (p *PacketEventProcessor) writeLoop() {
	defer close(p.wChan)
	var (
		err   error
		evs   = make([]*WriteEvent, p.maxBufferedTxEvents)
		buf   = make([][]byte, p.maxBufferedTxEvents)
		sizes = make([]int, p.maxBufferedTxEvents)
	)

	for {
		length := len(p.wChan)
		if length > 1 {
			for i := 0; i < length; i++ {
				event := <-p.wChan
				if event.N > 1 {
					p.wbChan <- event
				}
				evs[i] = event
				buf[i] = (*event.Buffer)[0]
				sizes[i] = (*event.Sizes)[0]
			}
			_, err = p.rw.BatchWrite(buf[:length], sizes[:length], p.headerSize)
			if err != nil && p.errHandler != nil {
				p.errHandler(ErrModuleTx, err)
			}
			for i := 0; i < length; i++ {
				p.PutTXEvent(evs[i])
			}
		} else {
			event := <-p.wChan
			if event.N > 1 {
				p.wbChan <- event
			} else {
				_, err = p.rw.Write(
					(*event.Buffer)[0][p.headerSize : (*event.Sizes)[0]+p.headerSize],
				)
				if err != nil && p.errHandler != nil {
					p.errHandler(ErrModuleTx, err)
				}
			}
			p.PutTXEvent(event)
		}
	}
}

func (p *PacketEventProcessor) writeBatchLoop() {
	defer close(p.wbChan)
	for event := range p.wbChan {
		_, err := p.rw.BatchWrite(
			(*event.Buffer)[:event.N],
			(*event.Sizes)[:event.N],
			p.headerSize)
		if err != nil && p.errHandler != nil {
			p.errHandler(ErrModuleTx, err)
		}
		p.PutTXEvent(event)
	}
}

func (p *PacketEventProcessor) PushWriteEvent(e *WriteEvent) {
	if e.N > 1 {
		p.wbChan <- e
	} else {
		p.wChan <- e
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
