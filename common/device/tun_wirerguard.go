package device

import (
	"runtime"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

type (
	// wireGuardDevice ...
	// needs dll in windows
	wireGuardDevice struct {
		device tun.Device

		offset     int
		readSizes  []int
		readBuffs  [][]byte
		writeBuffs [][]byte
		readMutex  sync.Mutex
		writeMutex sync.Mutex
	}
)

func newWireGuardDevice(config Config) (d *wireGuardDevice, err error) {
	d = &wireGuardDevice{
		readSizes:  make([]int, 1),
		readBuffs:  make([][]byte, 1),
		writeBuffs: make([][]byte, 1),
	}

	if runtime.GOOS == "unix" {
		d.offset = 4
	}

	// 尝试创建 TUN 设备
	d.device, err = tun.CreateTUN("tun%d", config.MTU)
	return
}

func (w *wireGuardDevice) Name() string {
	name, _ := w.device.Name()
	return name
}

func (w *wireGuardDevice) Close() error {
	return w.device.Close()
}

func (w *wireGuardDevice) Read(packet []byte) (n int, err error) {
	w.readMutex.Lock()
	defer w.readMutex.Unlock()
	w.readBuffs[0] = packet
	_, err = w.device.Read(w.readBuffs, w.readSizes, w.offset)
	return w.readSizes[0], err
}

func (w *wireGuardDevice) Write(packet []byte) (n int, err error) {
	w.writeMutex.Lock()
	defer w.writeMutex.Unlock()
	w.writeBuffs[0] = packet
	return w.device.Write(w.writeBuffs, w.offset)
}
