package device

import (
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

func newWireGuardDevice(config Config) (d *wireGuardDevice, err error) {
	d = &wireGuardDevice{
		bufferPool: sync.Pool{
			New: func() interface{} {
				return make([][]byte, 1)
			},
		},
		sizePool: sync.Pool{
			New: func() interface{} {
				return make([]int, 1)
			},
		},
	}

	// 尝试创建 TUN 设备
	d.device, err = tun.CreateTUN("tun%d", config.MTU)
	return
}
