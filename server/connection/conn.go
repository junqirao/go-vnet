package connection

import (
	"io"

	"go-vnet/common/device"
	"go-vnet/server/network"
)

type (
	Info struct {
		Network    *network.Network
		Device     *device.Device
		Src        string
		rwc        io.ReadWriteCloser
		GetRWCFunc func() io.ReadWriteCloser
	}
)

func (c *Info) GetRWC() io.ReadWriteCloser {
	if c.rwc != nil {
		return c.rwc
	}
	if c.GetRWCFunc == nil {
		return nil
	}
	c.rwc = c.GetRWCFunc()
	return c.rwc
}

func (c *Info) Close() error {
	if c.rwc != nil {
		return c.rwc.Close()
	}
	return nil
}

func (c *Info) SetRWC(rwc io.ReadWriteCloser) {
	c.rwc = rwc
}

func (c *Info) Clone() *Info {
	return &Info{
		Network:    c.Network,
		Device:     c.Device,
		Src:        c.Src,
		GetRWCFunc: c.GetRWCFunc,
	}
}
