package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/netip"
	"os/exec"
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

	batchSize := dev.BatchSize()
	headerSize := dev.FrontHeadroom()
	initReadPoolAndBuf(batchSize, headerSize)

	// todo up dev
	command := exec.Command("ip", "link", "set", "tun0", "up")
	err = command.Start()
	if err != nil {
		panic(err)
		return
	}

	// read loop
	go func() {
		fmt.Println("start tx")
		go handleTxReadDevice(tx, dev, c.dst)
		// go handleTxReadDevice(dev, headerSize, c.dst)
		// go handleTxWriteNetwork(tx, headerSize)
	}()

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
	batch := protocol.MaxTransportBatchSize

	var (
		buffers = make([][]byte, batch)
		sizes   = make([]int, batch)
	)

	for i := 0; i < batch; i++ {
		buffers[i] = make([]byte, mtu+headerSize)
	}

	for event := range p.RX() {
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

		p.PutRXEvent(event)
	}
}

func handleTxReadDevice(tx *quic.Stream, dev tun.LinuxTUN, dst string) {
	var (
		err        error
		headerSize = dev.FrontHeadroom()
		rw         = protocol.NewTransport(tx)
		p          = protocol.NewPacketEventProcessor(rw,
			protocol.WithMaxPacketSize(mtu+headerSize),
			protocol.WithHeaderSize(headerSize),
			protocol.WithBatchSize(dev.BatchSize()),
		)
	)

	for {
		event := p.GetTXEvent()
		event.N, err = dev.BatchRead(*event.Buffer, headerSize, *event.Sizes)
		if err != nil {
			panic(err)
			return
		}
		for i := 0; i < event.N; i++ {
			if waterutil.IPv4Destination((*event.Buffer)[i][headerSize:(*event.Sizes)[i]+headerSize]).String() != dst {
				// 移除 env.buf 和 env.sizes 中的元素
				event.DeleteElements(i)
			}
		}
		if event.N <= 0 {
			p.PutTXEvent(event)
			continue
		}

		p.PushWriteEvent(event)
	}
}
