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

	// read loop
	go func() {
		fmt.Println("start tx")
		rw := protocol.NewTransport(tx)
		dev, ok := c.device.dev.(tun.LinuxTUN)
		if !ok {
			panic("not linux tun")
		}

		batchSize := dev.BatchSize()
		bufs := make([][]byte, batchSize)
		headerSize := dev.FrontHeadroom()
		readN := make([]int, batchSize)
		for i := 0; i < batchSize; i++ {
			bufs[i] = make([]byte, mtu+headerSize)
		}

		for {
			n, err := dev.BatchRead(bufs, dev.FrontHeadroom(), readN)
			if err != nil {
				panic(err)
				return
			}
			// n, err := c.device.dev.Read(buf)
			// if err != nil {
			// 	panic(err)
			// 	return
			// }
			for i := 0; i < n; i++ {
				data := bufs[i][headerSize : readN[i]+headerSize]
				// [69 0 0 84 89 162 64 0 64 1 155 176 192 168 98 3 192 168 98 2 8 0 44 131 33 77 0 4 90 55 99 105 0 0 0 0 40 184 5 0 0 0 0 0 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45]
				// fmt.Println("read: ", data)
				dst := waterutil.IPv4Destination(data).String()
				// fmt.Println(dst)
				if dst != c.dst {
					continue
				}
				_, err = rw.Write(data)
				if err != nil {
					panic(err)
					return
				}
			}
		}
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
			go func(rx *quic.Stream) {
				rw := protocol.NewTransport(rx)
				p := protocol.NewPacketEventProcessor(rw)
				// buf := make([]byte, 1400)
				for event := range p.Ch() {
					_, err = c.device.dev.Write(event.Bytes())
					if err != nil {
						fmt.Println("receive: ", event.Bytes())
						panic(err)
						return
					}
					event.PutBack()
				}
				// for {
				// n, err := rw.Read(buf)
				// if err != nil {
				// 	panic(err)
				// 	return
				// }
				// fmt.Println("receive: ", buf[:n])
				// _, err = c.device.dev.Write(buf[:n])
				// if err != nil {
				// 	fmt.Println("receive: ", buf[:n])
				// 	panic(err)
				// 	return
				// }
				// }
			}(rx)
		}
	}()

	select {}
}
