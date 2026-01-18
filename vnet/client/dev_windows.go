package client

import (
	"context"
	"net/netip"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/vnet/session"
)

func (c *Client) setupDevice(ctx context.Context, sess *session.Session) (dev tun.Tun, err error) {
	defer func() {
		if err != nil {
			return
		}
		c.logger.Infof(ctx, "setup device success")
	}()

	if sess.DispatchedDevice.Name == "" {
		sess.DispatchedDevice.Name = "tun0"
	}

	c.logger.Infof(ctx, "setup device: name=%s, cidr=%s, mtu=%d, id=%s",
		sess.DispatchedDevice.Name,
		sess.DispatchedDevice.CIDR,
		sess.DispatchedDevice.MTU,
		sess.DispatchedDevice.Id)
	pfx, _ := netip.ParsePrefix(sess.DispatchedDevice.CIDR)
	dev, err = tun.New(tun.Options{
		Name:         sess.DispatchedDevice.Name,
		Inet4Address: []netip.Prefix{pfx},
		Inet6Address: nil,
		MTU:          uint32(sess.DispatchedDevice.MTU),
	})
	if err != nil {
		c.logger.Errorf(ctx, "create tun device error: %v", err.Error())
		return
	}
	return
}
