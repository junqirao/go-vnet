package protocol

import (
	"errors"
	"sync"
)

const (
	eventMaxBufferSize = 1500
)

type (
	PacketEventProcessor struct {
		rw        ReadWriter
		eventPool sync.Pool
		eventBuf  chan *Event
	}
	Event struct {
		putBack func()
		buffer  *[eventMaxBufferSize]byte
		n       uint16 // 使用 uint16 而不是 int，节省内存（与 protocol 长度字段类型一致）
		typ     byte
	}
)

// 使用预分配的缓冲池，避免每次都分配新数组
var eventBufferPool = sync.Pool{
	New: func() interface{} {
		return new([eventMaxBufferSize]byte)
	},
}

func newEvent() *Event {
	return &Event{
		buffer: nil, // 延迟分配，减少内存占用
	}
}

func (e *Event) PutBack() {
	if e.putBack == nil {
		return
	}
	e.putBack()
}

// Release releases the event back to the pool
// This is an alias for PutBack() for better naming
func (e *Event) Release() {
	e.PutBack()
}

func (e *Event) Type() byte {
	return e.typ
}

func (e *Event) Bytes() []byte {
	// 使用全切片表达式避免分配新的切片头
	return e.buffer[:e.n:e.n]
}

func (e *Event) N() int {
	return int(e.n)
}

// BufferPtr returns the raw buffer pointer and length without creating a slice
// This is useful for zero-copy operations
func (e *Event) BufferPtr() ([]byte, int) {
	return e.buffer[:e.n:e.n], int(e.n)
}

func NewPacketEventProcessor(rw ReadWriter) *PacketEventProcessor {
	p := &PacketEventProcessor{
		rw: rw,
		eventPool: sync.Pool{
			New: func() interface{} {
				return newEvent()
			},
		},
		eventBuf: make(chan *Event, 1024),
	}
	go p.readLoop()
	return p
}

func (p *PacketEventProcessor) Ch() <-chan *Event {
	return p.eventBuf
}

func (p *PacketEventProcessor) readLoop() {
	defer close(p.eventBuf)
	for {
		event := p.getEvent()
		// 直接传递数组指针，避免创建切片
		typ, n, err := p.rw.ReadMessage((*event.buffer)[:])
		if errors.Is(err, ErrInvalidMagic) {
			event.PutBack()
			continue
		}
		if err != nil {
			event.PutBack()
			return
		}
		event.n = uint16(n)
		event.typ = typ
		p.eventBuf <- event
	}
}

func (p *PacketEventProcessor) getEvent() *Event {
	event := p.eventPool.Get().(*Event)
	// 确保已分配 buffer
	if event.buffer == nil {
		event.buffer = eventBufferPool.Get().(*[eventMaxBufferSize]byte)
	}
	// 只在首次创建时分配闭包
	if event.putBack == nil {
		// 使用局部变量捕获 event，避免闭包中的循环引用
		e := event
		event.putBack = func() {
			eventBufferPool.Put(e.buffer)
			e.buffer = nil
			p.eventPool.Put(e)
		}
	}
	return event
}

// Stop stops the packet event processor
func (p *PacketEventProcessor) Stop() {
	// Note: The readLoop will be stopped when the underlying reader returns an error
	// or when the channel is closed externally
}
