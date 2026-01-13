package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
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

func (c *Client) Run() {
	go func() {
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()

	var (
		ctx = context.Background()
		err error
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

	// batchSize := dev.BatchSize()
	headerSize := dev.FrontHeadroom()

	// read loop
	// go handleTxBatch(tx, dev, batchSize, headerSize, c.dst)
	go handleTxReadDevice(dev, headerSize, c.dst)
	go handleTxWriteNetwork(tx)

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
			go handleRxBatch(rx, dev, headerSize)
		}
	}()

	select {}
}

func handleRxBatch(rx *quic.Stream, dev tun.LinuxTUN, headerSize int) {
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
			n, err := rw.ParseBatch(event.Bytes(), buffers, sizes, headerSize)
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

func handleTxReadDevice(dev tun.LinuxTUN, headerSize int, dst string) {
	var (
		err error
	)

	for {
		event := getDeviceReadEvent()
		event.n, err = dev.BatchRead(*event.buf, headerSize, *event.sizes)
		if err != nil {
			panic(err)
			return
		}
		for i := 0; i < event.n; i++ {
			if waterutil.IPv4Destination((*event.buf)[i][:(*event.sizes)[i]+headerSize]).String() != dst {
				// 移除 env.buf 和 env.sizes 中的元素
				event.deleteElements(i)
				continue
			}
		}
		if event.n <= 0 {
			putDeviceReadEvent(event)
			continue
		}
		readDeviceBuf <- event
	}
}

func handleTxWriteNetwork(tx *quic.Stream) {
	var (
		rw  = protocol.NewTransport(tx)
		err error
	)

	for event := range readDeviceBuf {
		if event.n > 1 {
			_, err = rw.BatchWrite((*event.buf)[:event.n], (*event.sizes)[:event.n], 0)
		} else {
			_, err = rw.Write((*event.buf)[0][:(*event.sizes)[0]])
		}
		if err != nil {
			panic(err)
			return
		}
		putDeviceReadEvent(event)
	}
}
