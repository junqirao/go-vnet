package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"go-vnet/vnet/client/hub"
	"go-vnet/vnet/server"
)

func (c *Client) syncRouter(ctx context.Context) (err error) {
	// ping
	resp, err := c.manager.CallFunc(ctx, server.FuncNamePing, map[string]any{
		"host_id": c.hostId,
	})
	if err != nil {
		c.logger.Errorf(ctx, "failed to execute ping to server: %s", err.Error())
		return
	}
	// c.logger.Infof(ctx, "ping latency: %.2fms", resp.Cost)

	// update router if hash changed
	data := strings.Split(resp.Data.(string), ",")
	current := c.hub.Router().MD5()
	remote := data[0]
	v, _ := strconv.Atoi(data[1])
	version := uint64(v)
	// sync router
	if remote != current {
		c.logger.Infof(ctx, "router hash changed, current: %s, server: %s", current, remote)
		if err := c.updateRouter(ctx); err != nil {
			c.logger.Errorf(ctx, "failed to update router: %s", err.Error())
		}
	}
	// sync relay
	if c.relayVersion.Load() != version {
		if err := c.syncRelay(ctx); err != nil {
			c.logger.Errorf(ctx, "failed to sync relay: %s", err.Error())
		}
		c.relayVersion.Store(version)
	}
	return
}

func (c *Client) syncRelay(ctx context.Context) (err error) {
	resp, err := c.manager.CallFunc(ctx, server.FuncNameGetP2PRelayMapping)
	if err != nil {
		c.logger.Errorf(ctx, "failed to execute get relay data from server: %s", err.Error())
		return
	}
	if data, ok := resp.Data.(string); ok && len(data) > 0 {
		var bs []byte
		bs, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			c.logger.Errorf(ctx, "failed to decode relay data from server: %s", err.Error())
			return
		}
		var mapping map[string]string
		err = json.Unmarshal(bs, &mapping)
		if err != nil {
			c.logger.Errorf(ctx, "failed to unmarshal relay data from server: %s", err.Error())
			return
		}

		upsert := 0
		del := 0
		for k, v := range mapping {
			c.relayMapping.Store(k, v)
			upsert++
		}
		c.relayMapping.Range(func(key, value any) bool {
			if _, ok := mapping[key.(string)]; !ok {
				c.relayMapping.Delete(key)
				del++
				c.logger.Infof(ctx, "remove relay: %s", key)
			}
			return true
		})
		c.logger.Infof(ctx, "relay synced from server, upsert: %d, delete: %d", upsert, del)
	}
	return
}

func (c *Client) updateRouter(ctx context.Context) (err error) {
	resp, err := c.manager.CallFunc(ctx, server.FuncNameGetRouterData)
	if err != nil {
		c.logger.Errorf(ctx, "failed to execute get router data from server: %s", err.Error())
		return
	}

	// decode and restore
	if data, ok := resp.Data.(string); ok && len(data) > 0 {
		var bs []byte
		bs, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			c.logger.Errorf(ctx, "failed to decode router data from server: %s", err.Error())
			return
		}

		var ips []string
		err = json.Unmarshal(bs, &ips)
		if err != nil {
			c.logger.Errorf(ctx, "failed to unmarshal router data from server: %s", err.Error())
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
				c.logger.Infof(ctx, "remove route: %s", cidr)
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
				c.logger.Infof(ctx, "add route: %s", ip)
				var hook hub.TxHook = c
				c.hub.Router().Register(ip,
					hub.NewDestination(context.Background(), strings.Split(ip, "/")[0], c.internal, c.hub, hook))
			}
		}

		c.logger.Infof(c.ctx, "router synced from server, data: %d bytes, length: %d",
			len(data), len(c.hub.Router().Keys()))
	}
	return
}

func (c *Client) syncRouterLoop() {
	c.state = StateRunning

	c.logger.Infof(c.ctx, "sync router loop started")
	defer c.logger.Infof(c.ctx, "sync router loop stopped")

	ticker := time.NewTicker(SyncRouterInterval)
	defer ticker.Stop()

	errCount := 0

	// sync router once
	_ = c.syncRouter(c.ctx)

	for {
		select {
		case <-c.sig:
			c.logger.Infof(c.ctx, "received shutdown signal")
			return
		case <-c.ctx.Done():
			c.logger.Infof(c.ctx, "context cancelled")
			return
		case <-ticker.C:
			// sync router with client context instead of Background
			if err := c.syncRouter(c.ctx); err != nil {
				c.logger.Errorf(c.ctx, "failed to sync router: %s", err.Error())
				errCount++
				if errCount >= MaxErrorToReconnect {
					ticker.Stop()
					c.logger.Infof(c.ctx, "max errors reached (%d), attempting reconnect", errCount)
					go c.reconnectLoop()
					return
				}
			} else {
				// reset error count on success
				errCount = 0
			}
		}
	}
}

func (c *Client) reconnectLoop() {
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
		c.logger.Infof(c.ctx, "trying to reconnect in %d seconds, tries: %d", interval, tries+1)

		// delay before reconnect to avoid busy loop
		time.Sleep(time.Duration(interval) * time.Second)

		// reconnect
		err := c.Reconnect()
		if err == nil {
			c.logger.Infof(c.ctx, "reconnected successfully")
			return
		}

		c.logger.Errorf(c.ctx, "failed to reconnect: %s", err.Error())
		tries++
		interval = fib(tries + 1)
		if interval > MaxReconnectInterval {
			interval = MaxReconnectInterval
		}
	}
}
