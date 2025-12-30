package device

import (
	"sync"

	"github.com/songgao/water"
	"golang.zx2c4.com/wireguard/tun"
)

const DefaultTunDeviceName = "default-tun-device"

type (
	// IDevice ...
	IDevice interface {
		Name() string                           // return device name if device exists
		Setup() error                           // create device
		Close() error                           // close device
		OverwriteCIDR(cidr string) error        // overwrite cidr
		OverwriteMTU(mtu int) error             // overwrite mtu
		Up() error                              // set device up
		Down() error                            // set device down
		Read(packet []byte) (n int, err error)  // read
		Write(packet []byte) (n int, err error) // write
		GetConfig() Config
	}
	// tunDevice ...
	tunDevice interface {
		Name() string                           // return device name if device exists
		Close() error                           // close device
		Read(packet []byte) (n int, err error)  // read
		Write(packet []byte) (n int, err error) // write
	}

	// waterDevice ...
	// supports only windows,linux,osx
	waterDevice struct {
		*water.Interface
	}
	// wireGuardDevice ...
	// needs dll in windows
	wireGuardDevice struct {
		device     tun.Device
		bufferPool sync.Pool
		sizePool   sync.Pool
	}
)

// NewTunDevice ...
func NewTunDevice(cfg Config, opts ...option) (device IDevice) {
	options := append(defaultOptions, WithConfig(cfg))
	d := newController(append(options, opts...)...)
	d.config = cfg
	return d
}

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
	d.device, err = tun.CreateTUN(config.Name, config.MTU)
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
	// Get a buffer from the pool
	var (
		sizes = w.sizePool.Get().([]int)
		buf   = w.bufferPool.Get().([][]byte)
	)
	buf[0] = packet

	defer func() {
		buf[0] = nil
		sizes[0] = 0
		w.bufferPool.Put(buf)
		w.sizePool.Put(sizes)
	}()

	// Read from the device
	_, err = w.device.Read(buf, sizes, 0)
	if err != nil {
		return 0, err
	}

	return sizes[0], nil
}

func (w *wireGuardDevice) Write(packet []byte) (n int, err error) {
	// Get a buffer from the pool
	buf := w.bufferPool.Get().([][]byte)
	defer func() {
		buf[0] = nil
		w.bufferPool.Put(buf)
	}()
	buf[0] = packet
	// Write to the device
	n, err = w.device.Write(buf, 0)
	return
}
