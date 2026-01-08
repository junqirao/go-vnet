package device

import (
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/context"
	"gvisor.dev/gvisor/pkg/tcpip/link/tun"
)

type (
	gVisorDevice struct {
		dev *tun.Device
	}
)

func newGVisorDevice() *gVisorDevice {
	return &gVisorDevice{
		dev: &tun.Device{},
	}
}

func (g *gVisorDevice) Name() string {
	return g.dev.Name()
}

func (g *gVisorDevice) Close() error {
	g.dev.Release(context.Background())
	return nil
}

func (g *gVisorDevice) Read(packet []byte) (n int, err error) {
	view, err := g.dev.Read()
	if err != nil {
		return 0, err
	}
	return view.Read(packet)
}

func (g *gVisorDevice) Write(packet []byte) (n int, err error) {
	write, err := g.dev.Write(buffer.NewViewWithData(packet))
	if err != nil {
		return 0, err
	}
	n = int(write)
	return
}
