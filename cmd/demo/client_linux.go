package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"time"

	"github.com/quic-go/quic-go"
	tun "github.com/sagernet/sing-tun"
	"github.com/songgao/water/waterutil"

	"go-vnet/common/config"
	"go-vnet/common/protocol"
	tt "go-vnet/common/tls"
)

var (
	mtu                   = 1392
	maxBatchTransportSize = 46
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

func NewClient(ip string, server string, dst string) *Client {
	return &Client{
		ip:     ip,
		server: server,
		dst:    dst,
	}
}

func (c *Client) Run() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	var (
		ctx = context.Background()
		err error
		// dc  = device.Config{
		// 	Name: "tun0",
		// 	CIDR: fmt.Sprintf("%s/24", c.ip),
		// 	MTU:  1400,
		// }
	)

	pfx, _ := netip.ParsePrefix(fmt.Sprintf("%s/24", c.ip))

	c.device.dev, err = tun.New(tun.Options{
		Name:         "tun0",
		Inet4Address: []netip.Prefix{pfx},
		Inet6Address: nil,
		MTU:          uint32(mtu),
		GSO:          true,
	})
	if err != nil {
		panic(err)
		return
	}

	// c.device.dev = device.NewTunDevice(dc)
	// if err = c.device.dev.Setup(); err != nil {
	// 	panic(err)
	// }

	tls := tt.GenerateTLSConfig(time.Hour*24*7, 1024)
	tls.InsecureSkipVerify = true
	c.transport.conn, err = quic.DialAddr(ctx, c.server, tls, config.DefaultQuicConfig)
	if err != nil {
		panic(err)
	}

	tx, err := c.transport.conn.OpenStreamSync(ctx)
	if err != nil {
		panic(err)
	}

	dev, ok := c.device.dev.(tun.LinuxTUN)
	if !ok {
		panic("not linux tun")
	}

	batchSize := dev.BatchSize()
	headerSize := dev.FrontHeadroom()

	// read loop
	go handleTxBatch(tx, dev, batchSize, headerSize, c.dst)

	// write loop
	go func() {
		fmt.Println("start rx")
		for {
			rx, err := c.transport.conn.AcceptStream(ctx)
			if err != nil {
				panic(err)
			}
			fmt.Println("accept stream")
			// go handleRx(rx, dev)
			go handleRxBatch2(rx, dev, batchSize, headerSize)
		}
	}()

	select {}
}

func handleRx(rx *quic.Stream, dev tun.Tun) {
	rw := protocol.NewTransport(rx)
	p := protocol.NewPacketEventProcessor(rw)

	for event := range p.Ch() {
		_, err := dev.Write(event.Bytes())
		if err != nil {
			panic(err)
			return
		}
		event.PutBack()
	}
}

func handleRxBatch(rx *quic.Stream, dev tun.LinuxTUN, batchSize int, headerSize int) {
	rw := protocol.NewTransport(rx)
	p := protocol.NewPacketEventProcessor(rw)

	var (
		evs     = make([]*protocol.Event, 0, batchSize)
		buffers = make([][]byte, batchSize)
	)

	// 预分配所有buffer
	for i := 0; i < batchSize; i++ {
		buffers[i] = make([]byte, mtu+headerSize)
	}

	for {
		evs = evs[:0]

		// 非阻塞收集一批数据，最多50微秒等待
		deadline := time.Now().Add(50 * time.Microsecond)
		for len(evs) < batchSize && time.Now().Before(deadline) {
			select {
			case ev := <-p.Ch():
				evs = append(evs, ev)
			default:
				// 如果没有立即可用数据，继续循环等待直到超时
				// 使用短sleep避免CPU空转
				time.Sleep(5 * time.Microsecond)
			}
		}

		// 如果没有收集到数据，阻塞等待第一个packet
		if len(evs) == 0 {
			evs = append(evs, <-p.Ch())
			// 尝试收集更多数据
			for len(evs) < batchSize && time.Now().Before(deadline) {
				select {
				case ev := <-p.Ch():
					evs = append(evs, ev)
				default:
					break
				}
			}
		}

		// 零值初始化header并复制数据
		for i := 0; i < len(evs); i++ {
			data := evs[i].Bytes()
			buf := buffers[i]
			for j := 0; j < headerSize; j++ {
				buf[j] = 0
			}
			copy(buf[headerSize:], data)
			buffers[i] = buf[:headerSize+len(data)]
		}

		// 批量写入
		_, err := dev.BatchWrite(buffers[:len(evs)], headerSize)
		if err != nil {
			panic(err)
			return
		}

		// PutBack 事件到池中
		for i := 0; i < len(evs); i++ {
			evs[i].PutBack()
		}
	}
}

func handleRxBatch2(rx *quic.Stream, dev tun.LinuxTUN, batchSize int, headerSize int) {
	rw := protocol.NewTransport(rx)
	p := protocol.NewPacketEventProcessor(rw)

	var (
		buffers = make([][]byte, 1024)
		sizes   = make([]int, 1024)
	)

	for i := 0; i < 1024; i++ {
		buffers[i] = make([]byte, mtu+headerSize)
	}

	for event := range p.Ch() {
		switch event.Type() {
		case protocol.TypeTransport:
			_, err := dev.Write(event.Bytes())
			if err != nil {
				panic(err)
				return
			}
		case protocol.TypeBatchTransport:
			n, err := rw.ParseBatch(event.Bytes(), buffers, sizes)
			if err != nil {
				return
			}
			_, err = dev.BatchWrite(buffers[:n], headerSize)
			if err != nil {
				panic(err)
				return
			}
		}

		event.PutBack()
	}
}

func handleTxBatch(tx *quic.Stream, dev tun.LinuxTUN, batchSize int, headerSize int, dst string) {
	fmt.Println("start tx")

	rw := protocol.NewTransport(tx)

	bufs := make([][]byte, batchSize)
	readN := make([]int, batchSize)
	for i := 0; i < batchSize; i++ {
		bufs[i] = make([]byte, mtu+headerSize)
	}

	for {
		n, err := dev.BatchRead(bufs, headerSize, readN)
		if err != nil {
			panic(err)
			return
		}
		if n > 1 {
			// 分批处理，最大批大小为 maxBatchTransportSize
			for i := 0; i < n; i += maxBatchTransportSize {
				end := i + maxBatchTransportSize
				if end > n {
					end = n
				}
				_, err = rw.BatchWrite(bufs[i:end], readN[i:end], headerSize)
				if err != nil {
					panic(err)
					return
				}
			}
		} else {
			data := bufs[0][headerSize : readN[0]+headerSize]
			if waterutil.IPv4Destination(data).String() != dst {
				continue
			}
			_, err = rw.Write(data)
			if err != nil {
				panic(err)
				return
			}
		}
	}
}
