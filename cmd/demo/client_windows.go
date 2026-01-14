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

	initReadPoolAndBuf(protocol.MaxTransportBatchSize, 0)

	// read loop
	go func() {
		fmt.Println("start tx")
		// go handleTxReadDevice(c.device.dev, c.dst)
		// go handleTxWriteNetwork(tx)
		go handleTxReadDeviceV2(tx, c.device.dev, c.dst)
		// handleTx(tx, c.device.dev, c.dst)
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
			go handleRx(rx, c.device.dev, 0)
		}
	}()

	select {}
}

func handleRx(rx *quic.Stream, dev tun.Tun, headerSize int) {
	rw := protocol.NewTransport(rx)
	p := protocol.NewPacketEventProcessor(rw)

	var (
		buffers = make([][]byte, 1024)
		sizes   = make([]int, 1024)
	)

	for i := 0; i < 1024; i++ {
		buffers[i] = make([]byte, mtu+headerSize)
	}

	for event := range p.RX() {
		switch event.Type() {
		case protocol.TypeTransport:
			_, err := dev.Write(event.Bytes())
			if err != nil {
				fmt.Println("receive: ", event.Bytes())
				panic(err)
				return
			}
		case protocol.TypeBatchTransport:
			n, err := rw.ParseBatch(event.Bytes(), buffers, sizes, 0)
			if err != nil {
				panic(err)
				return
			}
			for i := 0; i < n; i++ {
				_, err = dev.Write(buffers[i][:sizes[i]])
				if err != nil {
					panic(err)
					return
				}
			}
		}

		p.PutRXEvent(event)
	}
}

func handleTxReadDevice(dev tun.Tun, dst string) {
	for {
		event := getDeviceReadEvent()
		n, err := dev.Read((*event.buf)[0])
		if err != nil {
			return
		}
		if waterutil.IPv4Destination((*event.buf)[0][:n]).String() != dst {
			putDeviceReadEvent(event)
			continue
		}
		(*event.sizes)[0] = n
		event.n = 1
		readDeviceBuf <- event
	}
}

func handleTxWriteNetwork(tx *quic.Stream) {
	var (
		rw    = protocol.NewTransport(tx)
		err   error
		evs   = make([]*deviceReadEvent, 1024)
		bufs  = make([][]byte, 1024)
		sizes = make([]int, 1024)
	)

	for {
		length := len(readDeviceBuf)
		if length > 1 {
			for i := 0; i < length; i++ {
				evs[i] = <-readDeviceBuf
				bufs[i] = (*evs[i].buf)[0]
				sizes[i] = (*evs[i].sizes)[0]
			}

			_, err = rw.BatchWrite(bufs[:length], sizes[:length], 0)
			if err != nil {
				panic(err)
				return
			}

			for i := 0; i < length; i++ {
				putDeviceReadEvent(evs[i])
			}
		} else {
			event := <-readDeviceBuf
			_, err = rw.Write((*event.buf)[0][:(*event.sizes)[0]])
			if err != nil {
				panic(err)
				return
			}
			putDeviceReadEvent(event)
		}
	}
}

func handleTxReadDeviceV2(tx *quic.Stream, dev tun.Tun, dst string) {
	var (
		rw = protocol.NewTransport(tx)
		p  = protocol.NewPacketEventProcessor(rw,
			protocol.WithBatchSize(1),
			protocol.WithHeaderSize(0),
			protocol.WithMaxPacketSize(mtu))
	)

	for {
		event := p.GetTXEvent()
		n, err := dev.Read((*event.Buffer)[0])
		if err != nil {
			return
		}
		if waterutil.IPv4Destination((*event.Buffer)[0][:n]).String() != dst {
			p.PutTXEvent(event)
			continue
		}
		(*event.Sizes)[0] = n
		event.N = 1
		p.PushEvent(event)
	}
}
