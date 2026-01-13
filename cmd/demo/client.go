package main

import (
	"sync"

	"github.com/quic-go/quic-go"
	tun "github.com/sagernet/sing-tun"
)

const (
	maxHeaderSize         = 16
	maxBatchTransportSize = 46
)

var (
	devReadEventPool = sync.Pool{
		New: func() interface{} {
			e := &deviceReadEvent{}
			buf := make([][]byte, maxBatchTransportSize)
			for i := 0; i < maxBatchTransportSize; i++ {
				buf[i] = make([]byte, maxHeaderSize+mtu)
			}
			e.buf = &buf
			sizes := make([]int, maxBatchTransportSize)
			e.sizes = &sizes
			return e
		},
	}
	readDeviceBuf = make(chan *deviceReadEvent, 1024)
)

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
		e.n--
	}

	// 更新有效长度
	// *e.sizes = (*e.sizes)[:writeIdx]
	// *e.buf = (*e.buf)[:writeIdx]
}

// sortInts 原地排序整数切片（内联快速排序，避免调用 runtime.sort）
func sortInts(a []int) {
	if len(a) < 2 {
		return
	}
	quickSortInts(a, 0, len(a)-1)
}

func quickSortInts(a []int, lo, hi int) {
	for lo < hi {
		p := partitionInts(a, lo, hi)
		if p-lo < hi-p {
			quickSortInts(a, lo, p-1)
			lo = p + 1
		} else {
			quickSortInts(a, p+1, hi)
			hi = p - 1
		}
	}
}

func partitionInts(a []int, lo, hi int) int {
	pivot := a[hi]
	i := lo - 1
	for j := lo; j < hi; j++ {
		if a[j] <= pivot {
			i++
			a[i], a[j] = a[j], a[i]
		}
	}
	a[i+1], a[hi] = a[hi], a[i+1]
	return i + 1
}
