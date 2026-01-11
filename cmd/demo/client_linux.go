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
	mtu = 1392
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
			go handleRx(rx, dev)
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
		evs     []*protocol.Event
		buffers = make([][]byte, 1024)
		l       int
		header  = make([]byte, headerSize)
	)

	for i := 0; i < batchSize; i++ {
		buffers[i] = make([]byte, mtu+headerSize)
	}

	for {
		// 重置 evs slice
		evs = evs[:0]

		// 获取当前可用的 event 数量
		if l = len(p.Ch()); l > 0 {
			for i := 0; i < l; i++ {
				evs = append(evs, <-p.Ch())
			}
		} else {
			// wait for packet
			evs = append(evs, <-p.Ch())
		}

		// 不分批，直接处理所有 evs
		for i, ev := range evs {
			data := ev.Bytes()
			buf := buffers[i]
			copy(buf[:headerSize], header)
			copy(buf[headerSize:], data)
			buffers[i] = buf[:headerSize+len(data)]
		}

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

func handleTxBatch(tx *quic.Stream, dev tun.LinuxTUN, batchSize int, headerSize int, dst string) {
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
		for i := 0; i < n; i++ {
			data := bufs[i][headerSize : readN[i]+headerSize]
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
