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
	go func() {
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
				var (
					evs     []*protocol.Event
					buffers = make([][]byte, batchSize)
					l       int
				)
				// 初始化 buffers，预留 headerSize 空间
				for i := 0; i < batchSize; i++ {
					buffers[i] = make([]byte, mtu+headerSize)
				}

				for {
					// 重置 evs slice，重用底层数组避免分配
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

					// 分批处理 evs
					evsLen := len(evs)
					for offset := 0; offset < evsLen; {
						batchEnd := min(offset+batchSize, evsLen)
						batchLen := batchEnd - offset

						for i := 0; i < batchLen; i++ {
							// 从 headerSize 位置开始复制数据
							data := evs[offset+i].Bytes()
							copy(buffers[i][headerSize:], data)
							buffers[i] = buffers[i][:headerSize+len(data)]
						}

						_, err := dev.BatchWrite(buffers[:batchLen], headerSize)
						if err != nil {
							panic(err)
							return
						}

						// PutBack 当前批次的事件到池中
						for i := 0; i < batchLen; i++ {
							evs[offset+i].PutBack()
						}

						offset += batchLen
					}
				}
			}(rx)
		}
	}()

	select {}
}
