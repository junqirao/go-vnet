package main

import (
	"sync"

	"github.com/quic-go/quic-go"
	tun "github.com/sagernet/sing-tun"
)

var (
	devReadEventPool = sync.Pool{}
	readDeviceBuf    = make(chan *deviceReadEvent, 1024)
)

func initReadPoolAndBuf(batchSize, headerSize int) {
	devReadEventPool = sync.Pool{
		New: func() interface{} {
			e := &deviceReadEvent{}
			buf := make([][]byte, batchSize)
			for i := 0; i < batchSize; i++ {
				buf[i] = make([]byte, headerSize+mtu)
			}
			e.buf = &buf
			sizes := make([]int, batchSize)
			e.sizes = &sizes
			return e
		},
	}
	readDeviceBuf = make(chan *deviceReadEvent, 1024)
}

type Client struct {
	ip, server, dst string
	device          struct {
		// dev device.IDevice
		dev tun.Tun
	}
	transport struct {
		conn *quic.Conn
	}
}

type deviceReadEvent struct {
	buf   *[][]byte
	sizes *[]int
	n     int
}

func getDeviceReadEvent() *deviceReadEvent {
	return devReadEventPool.Get().(*deviceReadEvent)
}

func putDeviceReadEvent(e *deviceReadEvent) {
	devReadEventPool.Put(e)
}

func NewClient(ip string, server string, dst string) *Client {
	return &Client{
		ip:     ip,
		server: server,
		dst:    dst,
	}
}

func (e *deviceReadEvent) deleteElements(i ...int) {
	if len(i) == 0 || len(i) >= len(*e.sizes) {
		return
	}

	// 对删除索引排序（原地快速排序）
	// sortInts(i)

	// 双指针：writeIdx 写入位置，skipIdx 跳过索引位置
	skipIdx := 0
	writeIdx := 0
	totalLen := len(*e.sizes)

	for readIdx := 0; readIdx < totalLen; readIdx++ {
		// 检查当前索引是否需要跳过
		if skipIdx < len(i) && readIdx == i[skipIdx] {
			skipIdx++
			continue
		}
		// 如果不是当前位置才移动
		if readIdx != writeIdx {
			(*e.buf)[writeIdx] = (*e.buf)[readIdx]
			(*e.sizes)[writeIdx] = (*e.sizes)[readIdx]
		}
		writeIdx++
		// 更新有效长度
		e.n--
	}
}
