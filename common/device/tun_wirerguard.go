package device

import (
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

type (
	// wireGuardDevice ...
	// needs dll in windows
	wireGuardDevice struct {
		device  tun.Device
		readBuf chan *wireGuardDevicePkg
	}
	wireGuardDevicePkg struct {
		data []byte
		n    int
	}
)

func newWireGuardDevice(config Config) (d *wireGuardDevice, err error) {
	d = &wireGuardDevice{
		readBuf: make(chan *wireGuardDevicePkg, 1024),
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
	// 先从缓冲区读取
	if len(w.readBuf) > 0 {
		pkg := <-w.readBuf
		copy(packet, pkg.data[:pkg.n])
		return pkg.n, nil
	}

	// 缓冲区为空，调用 read() 填充缓冲区
	_, err = w.read()
	if err != nil {
		return 0, err
	}

	// 从填充后的缓冲区读取
	if len(w.readBuf) > 0 {
		pkg := <-w.readBuf
		copy(packet, pkg.data[:pkg.n])
		return pkg.n, nil
	}

	return 0, nil
}

func (w *wireGuardDevice) read() (n int, err error) {
	var (
		sizes  = make([]int, w.device.BatchSize())
		buf    = make([][]byte, w.device.BatchSize())
		offset = device.MessageTransportHeaderSize
	)

	for i := range buf {
		buf[i] = make([]byte, 65535)
	}

	// Read from the device
	n, err = w.device.Read(buf, sizes, offset)
	if err != nil {
		return 0, err
	}

	for i := 0; i < n; i++ {
		w.readBuf <- &wireGuardDevicePkg{
			data: buf[i],
			n:    sizes[i],
		}
	}
	return n, nil
}

func (w *wireGuardDevice) Write(packet []byte) (n int, err error) {
	// WireGuard TUN 设备的 Write 函数需要 [][]byte 类型的参数
	// 检查 packet 是否为空
	if len(packet) == 0 {
		return 0, nil
	}
	var (
		offset = device.MessageTransportHeaderSize
	)

	return w.device.Write([][]byte{packet}, offset)
}
