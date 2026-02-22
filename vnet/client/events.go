package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/libp2p/go-libp2p/core/peer"

	"go-vnet/vnet/client/hub"
	"go-vnet/vnet/server"
)

func (c *Client) serverEventHandler(ctx context.Context, event *server.ServersideEvent) {
	g.Log().Infof(ctx, "receive server event: name=%s,id=%v,data=%v", event.Event, event.EventId, event.Data)
	switch event.Event {
	case server.EventNameQuotaUsageUpdate:
		c.handleQuotaUsageUpdateEvent(ctx, event)
	case server.EventNameP2PPeerUpdate, server.EventNameP2PPeerDelete:
		c.handleP2PPeerEvent(ctx, event)
	case server.EventNameRouterUpdate, server.EventNameRouterDelete:
		c.handleRouterEvent(ctx, event)
	default:
		g.Log().Infof(ctx, "dropped unknown event: %s", event.Event)
	}
}

func (c *Client) handleQuotaUsageUpdateEvent(_ context.Context, event *server.ServersideEvent) {
	old := c.session.DispatchedDevice.DataTrafficUsed
	c.session.DispatchedDevice.DataTrafficUsed = gconv.Int64(event.Data)
	g.Log().Infof(c.session.Ctx, "quota usage update: %d -> %d", old, c.session.DispatchedDevice.DataTrafficUsed)
}

func (c *Client) handleP2PPeerEvent(ctx context.Context, event *server.ServersideEvent) {
	data := &server.P2PPeerEventData{}
	if err := gconv.Struct(event.Data, data); err != nil {
		g.Log().Errorf(ctx, "parse p2p peer event data error: %s", err.Error())
		return
	}
	switch event.Event {
	case server.EventNameP2PPeerUpdate:
		pi := &peer.AddrInfo{}
		if err := json.Unmarshal([]byte(data.Peer), pi); err != nil {
			g.Log().Infof(ctx, "failed to parse p2p address: %v", err)
			return
		}
		g.Log().Infof(ctx, "p2p peer update: %s,%s", data.Ip, data.Peer)
		c.p2p.peerMapping.Store(data.Ip, pi)
	case server.EventNameP2PPeerDelete:
		g.Log().Infof(ctx, "p2p peer delete: %s", data.Peer)
		c.p2p.peerMapping.Delete(data.Ip)
	}
}

func (c *Client) handleRouterEvent(ctx context.Context, event *server.ServersideEvent) {
	ip := gconv.String(event.Data)
	cidr := fmt.Sprintf("%s/32", ip)
	switch event.Event {
	case server.EventNameRouterUpdate:
		_, ok := c.hub.Router().RouteString(ip)
		if ok {
			// ignore
			g.Log().Infof(ctx, "ignore exist router: %s", ip)
			return
		}
		// register empty route
		var hook hub.TxHook = c
		dst := hub.NewDestination(context.Background(), strings.Split(ip, "/")[0], c.internal, c.hub, hook)
		dst.SetP2PDialFunc(func(ctx context.Context) error {
			return c.tryP2P(ctx, dst)
		})
		c.hub.Router().Register(cidr, dst)
		g.Log().Infof(ctx, "router created: %s", cidr)
	case server.EventNameRouterDelete:
		c.hub.Router().UnRegister(cidr)
		g.Log().Infof(ctx, "router delete: %s", cidr)
	}
}
