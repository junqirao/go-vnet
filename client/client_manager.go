package client

import (
	"encoding/json"
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

		// ping
		data, err := c.cm.ExecFunc(server.FuncNamePing, nil)
		if err != nil {
			c.logger.Errorf(c.ctx, "failed to execute ping to server: %s", err.Error())
			continue
		}
		var resp server.FuncCallResponse
		if err = json.Unmarshal(data, &resp); err != nil {
			c.logger.Errorf(c.ctx, "failed to unmarshal ping response: %s", err.Error())
			continue
		}
		c.logger.Infof(c.ctx, "ping response: %+v", resp)
		// get router hash

		// sleep interval
		time.Sleep(time.Second * 5)
	}
}
