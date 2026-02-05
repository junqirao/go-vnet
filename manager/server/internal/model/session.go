package model

import (
	"time"

	"go-vnet/common/metrics"
	"go-vnet/common/session"
	"go-vnet/vnet/server"
)

type (
	Session struct {
		SessionId        string                    `json:"session_id"`
		Network          string                    `json:"network"`
		IP               string                    `json:"ip"`
		Type             session.Type              `json:"type"`
		ClientInfo       server.ClientInfo         `json:"client_info"`
		DispatchedDevice session.Device            `json:"dispatched_device"`
		Metrics          *metrics.TransportMetrics `json:"metrics"`
		CreatedAt        time.Time                 `json:"created_at"`
	}
)
