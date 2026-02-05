package client

import (
	"context"
	"net/netip"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/sagernet/netlink"
	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/session"
)

func (c *Client) setupDevice(ctx context.Context, sess *session.Session) (dev tun.Tun, err error) {
	defer func() {
		if err != nil {
			return
		}
		g.Log().Infof(ctx, "setup device success")
	}()

	if sess.DispatchedDevice.Name == "" {
		sess.DispatchedDevice.Name = "tun0"
	}

	g.Log().Infof(ctx, "setup device: name=%s, cidr=%s, mtu=%d, id=%s",
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
		GSO:          true,
	})
	if err != nil {
		g.Log().Errorf(ctx, "create tun device error: %v", err.Error())
		return
	}

	var link netlink.Link
	link, err = netlink.LinkByName(sess.DispatchedDevice.Name)
	if err != nil {
		g.Log().Errorf(ctx, "get tun device error: %v", err.Error())
		return
	}
	if err = netlink.LinkSetUp(link); err != nil {
		g.Log().Errorf(ctx, "set tun device up error: %v", err.Error())
		return
	}
	return
}
