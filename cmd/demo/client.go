package main

import (
	"context"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/songgao/water/waterutil"
	device2 "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"

	"go-vnet/common/config"
	"go-vnet/common/device"
	tt "go-vnet/common/tls"
)

type Client struct {
	ip, server, dst string
	device          struct {
		dev        tun.Device
		controller *device.Controller
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

	c.device.dev, err = tun.CreateTUN(dc.Name, dc.MTU)
	if err != nil {
		panic(err)
	}

	c.device.controller = device.NewController(device.WithConfig(dc), device.WithTunDevice(&td{c.device.dev}))
	err = c.device.controller.SetupProperties()
	if err != nil {
		panic(err)
	}
	fmt.Printf("device setup: %+v\n", c.device.controller.GetConfig())

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

		batchSize := c.device.dev.BatchSize()
		buffers := make([][]byte, batchSize)
		sizes := make([]int, batchSize)
		offset := device2.MessageTransportHeaderSize

		for i := 0; i < batchSize; i++ {
			buffers[i] = make([]byte, 65535)
		}

		for {
			n, err := c.device.dev.Read(buffers, sizes, offset)
			if err != nil {
				panic(err)
				return
			}

			for i := 0; i < n; i++ {
				data := buffers[i][:+sizes[i]]
				dst := waterutil.IPv4Destination(data[offset:]).String()
				if dst != c.dst {
					continue
				}
				_, err := tx.Write(data)
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
				buf := make([]byte, 65535)
				for {
					n, err := rx.Read(buf)
					if err != nil {
						panic(err)
						return
					}
					fmt.Println("receive: ", buf[:n])
					_, err = c.device.dev.Write([][]byte{buf[:n]}, device2.MessageTransportHeaderSize)
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
