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

	// read loop
	go func() {
		fmt.Println("start tx")
		handleTx(tx, c.device.dev, c.dst)
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

	for event := range p.Ch() {
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

		event.PutBack()
	}
}

func handleTx(tx *quic.Stream, dev tun.Tun, dst string) {
	buf := make([]byte, mtu)
	rw := protocol.NewTransport(tx)
	for {
		n, err := dev.Read(buf)
		if err != nil {
			panic(err)
			return
		}

		data := buf[:n]
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
