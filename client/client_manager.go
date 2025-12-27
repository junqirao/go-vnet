package client

import (
	"encoding/base64"
	"time"

	"go-vnet/server"
)

func (c *Client) startManager() {
	for {
		select {
		case <-c.sig:
			c.logger.Infof(c.ctx, "manager connection closed.")
			return
		default:
		}

		// sync router
		if err := c.SyncRouter(); err != nil {
			c.logger.Errorf(c.ctx, "failed to sync router: %s", err.Error())
		}

		// sleep interval
		time.Sleep(time.Second * 5)
	}
}

func (c *Client) SyncRouter() (err error) {
	// ping
	resp, err := c.cm.ExecFunc(server.FuncNamePing)
	if err != nil {
		c.logger.Errorf(c.ctx, "failed to execute ping to server: %s", err.Error())
		return
	}

	// update router if hash changed
	current := c.router.Hash()
	if resp.Data == current {
		return
	}

	// get router data from server
	c.logger.Infof(c.ctx, "router hash changed, current: %s, server: %s", current, resp.Data)
	resp, err = c.cm.ExecFunc(server.FuncNameGetRouterData)
	if err != nil {
		c.logger.Errorf(c.ctx, "failed to execute get router data from server: %s", err.Error())
		return
	}

	// decode and restore
	if data, ok := resp.Data.(string); ok && len(data) > 0 {
		var bs []byte
		bs, err = base64.StdEncoding.DecodeString(data)
		if err != nil {
			c.logger.Errorf(c.ctx, "failed to decode router data from server: %s", err.Error())
			return
		}
		if err = c.router.Restore(c.ctx, bs); err != nil {
			c.logger.Errorf(c.ctx, "failed to restore router from server: %s", err.Error())
			return
		}

		c.logger.Infof(c.ctx, "router synced from server, data: %d bytes, length: %d",
			len(data), c.router.Len())
	}
	return
}
