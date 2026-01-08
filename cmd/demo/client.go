package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/songgao/water/waterutil"
	device2 "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"go-vnet/common/config"
	"go-vnet/common/device"
	tt "go-vnet/common/tls"
)

var (
	offset = device2.MessageTransportHeaderSize
)

func init() {
	if runtime.GOOS == "windows" {
		offset = 0
	}
}

type Client struct {
	ip, server, dst string
	device          struct {
		dev device.IDevice
	}
	transport struct {
		conn *quic.Conn
	}
}

type td struct {
	dev tun.Device
}

func (t td) Name() string {
	name, _ := t.dev.Name()
	return name
}

func (t td) Close() error {
	return t.dev.Close()
}

func (t td) Read(packet []byte) (n int, err error) {
	return
}

func (t td) Write(packet []byte) (n int, err error) {
	return
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
		dc  = device.Config{
			Name: "tun0",
			CIDR: fmt.Sprintf("%s/24", c.ip),
			MTU:  1400,
		}
	)

	c.device.dev = device.NewTunDevice(dc)
	if err = c.device.dev.Setup(); err != nil {
		panic(err)
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

	// read loop
	go func() {
		fmt.Println("start tx")
		buf := make([]byte, 65535)
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
				buf := make([]byte, 65535)
				for {
					n, err := rx.Read(buf)
					if err != nil {
						panic(err)
						return
					}
					// fmt.Println("receive: ", buf[:n])
					_, err = c.device.dev.Write(buf[:n])
					if err != nil {
						panic(err)
						return
					}
				}
			}(rx)
		}
	}()

	select {}
}
