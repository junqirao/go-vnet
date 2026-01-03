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
	// Windows 上 WireGuard TUN 驱动使用不同的参数
	d.device, err = tun.CreateTUN(config.Name, config.MTU)
	return
}
