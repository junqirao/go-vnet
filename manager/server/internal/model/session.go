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
		DispatchedDevice session.Device            `json:"dispatched_device"`
		Metrics          *metrics.TransportMetrics `json:"metrics"`
		CreatedAt        time.Time                 `json:"created_at"`
	}
	Network struct {
		Config  server.NetworkConfig      `json:"config"`
		Metrics *metrics.TransportMetrics `json:"metrics"`
	}
	NetworkDetails struct {
		Network      *Network   `json:"network"`
		Sessions     []*Session `json:"sessions"`
		SessionCount int        `json:"session_count"`
	}
)
