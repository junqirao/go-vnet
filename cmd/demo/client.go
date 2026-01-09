package main

import (
	"context"
	"fmt"
	"net/netip"
	"time"

	"github.com/quic-go/quic-go"
	tun "github.com/sagernet/sing-tun"
	"github.com/songgao/water/waterutil"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
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
		MTU:          1400,
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
		buf := make([]byte, 1400)
		for {
			n, err := c.device.dev.Read(buf)
			if err != nil {
				panic(err)
				return
			}

			data := buf[:n]
			dst := waterutil.IPv4Destination(data).String()
			if dst != c.dst {
				continue
			}
			_, err = tx.Write(data)
			if err != nil {
				panic(err)
				return
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
				buf := make([]byte, 1400)
				for {
					n, err := rx.Read(buf)
					if err != nil {
						panic(err)
						return
					}
					// fmt.Println("receive: ", buf[:n])
					_, err = c.device.dev.Write(buf[:n])
					if err != nil {
						fmt.Println("receive: ", buf[:n])
						panic(err)
						return
					}
				}
			}(rx)
		}
	}()

	select {}
}
