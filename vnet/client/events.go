package client

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"go-vnet/vnet/server"
)

func (c *Client) serverEventHandler(ctx context.Context, event *server.ServersideEvent) {
	g.Log().Infof(ctx, "receive server event: name=%s,id=%v,data=%v", event.Event, event.EventId, event.Data)
	switch event.Event {
	case server.EventNameQuotaUsageUpdate:
		c.handleQuotaUsageUpdateEvent(ctx, event)
	default:
		g.Log().Infof(ctx, "dropped unknown event: %s", event.Event)
	}
}

func (c *Client) handleQuotaUsageUpdateEvent(_ context.Context, event *server.ServersideEvent) {
	old := c.session.DispatchedDevice.DataTrafficUsed
	c.session.DispatchedDevice.DataTrafficUsed = gconv.Int64(event.Data)
	g.Log().Infof(c.session.Ctx, "quota usage update: %d -> %d", old, c.session.DispatchedDevice.DataTrafficUsed)
}
