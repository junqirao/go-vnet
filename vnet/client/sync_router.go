package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/libp2p/go-libp2p/core/peer"

	"go-vnet/vnet/client/hub"
	"go-vnet/vnet/server"
)

func (c *Client) heartbeatAndSync(ctx context.Context) (err error) {
	// check if manager is initialized
	if c.manager == nil {
		return nil
	}
	// ping
	resp, err := c.manager.CallFunc(ctx, server.FuncNamePing)
	if err != nil {
		g.Log().Errorf(ctx, "failed to execute ping to server: %s", err.Error())
		return
	}
	// update latency
	c.ping.latencyToServer = fmt.Sprintf("%.2fms", resp.Cost)
	c.ping.lastHeartbeat = time.Now()

	// update router if hash changed
	data := strings.Split(resp.Data.(string), ",")
	current := c.hub.Router().MD5()
	remote := data[0]
	// sync router
	if remote != current {
		g.Log().Infof(ctx, "router hash changed, current: %s, server: %s", current, remote)
		if err := c.updateRouter(ctx); err != nil {
			g.Log().Errorf(ctx, "failed to update router: %s", err.Error())
		}
	}

	if !c.cfg.P2P.Enabled {
		return
	}
	// sync p2p peers
	current = c.p2p.router.MD5()
	remote = data[1]
	if remote != current {
		g.Log().Infof(ctx, "p2p peer router mismatch, current: %s, server: %s", current, remote)
		if err := c.syncP2PPeerMapping(ctx); err != nil {
			g.Log().Errorf(ctx, "failed to sync peer mapping: %s", err.Error())
		}
	}
	return
}

func (c *Client) syncP2PPeerMapping(ctx context.Context) (err error) {
	if !c.cfg.P2P.Enabled {
		return nil
	}
	// check if manager is initialized
	if c.manager == nil {
		return nil
	}
	resp, err := c.manager.CallFunc(ctx, server.FuncNameGetP2PPeerMapping)
	if err != nil {
		g.Log().Errorf(ctx, "failed to execute get peer mapping data from server: %s", err.Error())
		return
	}
	if data, ok := resp.Data.(string); ok && len(data) > 0 {
		var bs []byte
		bs, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			g.Log().Errorf(ctx, "failed to decode peer mapping data from server: %s", err.Error())
			return
		}
		var mapping map[string]string
		err = json.Unmarshal(bs, &mapping)
		if err != nil {
			g.Log().Errorf(ctx, "failed to unmarshal peer mapping data from server: %s", err.Error())
			return
		}

		upsert := 0
		del := 0
		for k, v := range mapping {
			if k == c.session.IP {
				continue
			}
			pi := &peer.AddrInfo{}
			if err = json.Unmarshal([]byte(v), pi); err != nil {
				g.Log().Infof(ctx, "failed to parse p2p address: %v", err)
				continue
			}

			// ignore exists
			if v, ok := c.p2p.router.RouteString(k); ok {
				curr := v.(*peer.AddrInfo)
				if curr.ID == c.p2p.host.ID() || curr.ID == pi.ID {
					continue
				}
			}
			// don't ignore self otherwise will cause an update loop
			// md5 will never be the same, it's ok to leave it here
			c.p2p.router.Register(k, pi)
			g.Log().Infof(ctx, "add p2p peer: %s", k)
			upsert++
		}
		toDel := map[string]struct{}{}
		c.p2p.router.Range(func(key string, value any) {
			if _, ok := mapping[key]; !ok {
				toDel[key] = struct{}{}
			}
		})
		for key := range toDel {
			c.p2p.router.UnRegister(key)
			del++
			conn, ok := c.p2p.connections.Load(key)
			if ok && conn != nil {
				conn.(*p2pConnInfo).cancel()
			}
			g.Log().Infof(ctx, "remove peer: %s", key)
		}

		g.Log().Infof(ctx, "synced p2p peer mapping from server, version: %s, upsert: %d, delete: %d",
			c.p2p.router.MD5(), upsert, del)
	}
	return
}

func (c *Client) updateRouter(ctx context.Context) (err error) {
	// check if manager is initialized
	if c.manager == nil {
		return nil
	}
	resp, err := c.manager.CallFunc(ctx, server.FuncNameGetRouterData)
	if err != nil {
		g.Log().Errorf(ctx, "failed to execute get router data from server: %s", err.Error())
		return
	}

	// decode and restore
	if data, ok := resp.Data.(string); ok && len(data) > 0 {
		var bs []byte
		bs, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			g.Log().Errorf(ctx, "failed to decode router data from server: %s", err.Error())
			return
		}

		var ips []string
		err = json.Unmarshal(bs, &ips)
		if err != nil {
			g.Log().Errorf(ctx, "failed to unmarshal router data from server: %s", err.Error())
			return
		}
		mip := map[string]struct{}{}
		for _, ip := range ips {
			mip[ip] = struct{}{}
		}
		cip := map[string]struct{}{}
		for _, ip := range c.hub.Router().Keys() {
			cip[ip] = struct{}{}
		}

		for cidr := range cip {
			_, ok = mip[cidr]
			if !ok {
				g.Log().Infof(ctx, "remove route: %s", cidr)
				ip := strings.Split(cidr, "/")[0]
				v, ok := c.hub.Router().RouteString(ip)
				if ok {
					c.hub.Router().UnRegister(cidr)
				}
				if dst, ok := v.(*hub.Destination); ok {
					_ = dst.Close()
				}
				c.internal.CloseDst(ctx, ip)
			}
		}
		for ip := range mip {
			_, ok = cip[ip]
			if !ok {
				if strings.HasPrefix(ip, c.session.IP) {
					c.hub.Router().Register(ip, nil)
					continue
				}
				g.Log().Infof(ctx, "add route: %s", ip)
				var hook hub.TxHook = c
				dst := hub.NewDestination(context.Background(), strings.Split(ip, "/")[0], c.internal, c.hub, hook)
				dst.SetP2PDialFunc(func(ctx context.Context) error {
					return c.tryP2P(ctx, dst)
				})
				c.hub.Router().Register(ip, dst)
			}
		}

		g.Log().Infof(c.ctx, "router synced from server, data: %d bytes, length: %d",
			len(data), len(c.hub.Router().Keys()))
	}
	return
}

func (c *Client) heartbeatAndSyncLoop() {
	c.state = StateRunning

	g.Log().Infof(c.ctx, "sync router loop started")
	defer g.Log().Infof(c.ctx, "sync router loop stopped")

	heartbeat := time.NewTicker(HeartbeatInterval)
	defer func() {
		heartbeat.Stop()
	}()

	c.ping.errorCount = 0

	// do first sync
	if err := c.heartbeatAndSync(c.ctx); err != nil {
		g.Log().Errorf(c.ctx, "failed to sync router: %s", err.Error())
	}

	for {
		select {
		case <-c.sig:
			g.Log().Infof(c.ctx, "received shutdown signal")
			return
		case <-c.ctx.Done():
			g.Log().Infof(c.ctx, "context cancelled")
			return
		case <-heartbeat.C:
			// check if client is stopped before syncing router
			if c.state == StateStopped {
				return
			}
			// sync router with client context instead of Background
			if err := c.heartbeatAndSync(c.ctx); err != nil {
				g.Log().Errorf(c.ctx, "failed to sync router: %s", err.Error())
				c.ping.errorCount++
				if c.ping.errorCount >= MaxErrorToReconnect {
					heartbeat.Stop()
					g.Log().Infof(c.ctx, "max errors reached (%d), attempting reconnect", c.ping.errorCount)
					go c.reconnectLoop()
					return
				}
			} else {
				// reset error count on success
				c.ping.errorCount = 0
			}
		}
	}
}

func (c *Client) reconnectLoop() {
	if c.state == StateStopped {
		return
	}
	g.Log().Infof(c.ctx, "reconnect loop started")
	c.state = StateReconnecting
	var (
		tries    = 0
		interval = 1
	)

	// 预计算的斐波那契数列（避免递归重复计算）
	fib := func(n int) int {
		if n <= 0 {
			return 1
		}
		a, b := 1, 1
		for i := 2; i <= n; i++ {
			a, b = b, a+b
		}
		return b
	}

	for {
		g.Log().Infof(c.ctx, "trying to reconnect in %d seconds, tries: %d", interval, tries+1)

		// delay before reconnect to avoid busy loop
		time.Sleep(time.Duration(interval) * time.Second)

		// stop loop if client is stopped or running
		if c.state == StateStopped || c.state == StateRunning {
			return
		}
		// reconnect
		err := c.Reconnect(context.WithValue(context.Background(), "retry", true))
		if err == nil {
			g.Log().Infof(c.ctx, "reconnected successfully")
			return
		}

		g.Log().Errorf(c.ctx, "failed to reconnect: %s", err.Error())
		tries++
		interval = fib(tries + 1)
		if interval > MaxReconnectInterval {
			interval = MaxReconnectInterval
		}
		c.ping.reconnectInterval = interval
		c.ping.retires = tries
	}
}
