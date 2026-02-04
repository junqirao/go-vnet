package model

import (
	"time"

	"go-vnet/common/metrics"
	"go-vnet/manager/server/internal/model/entity"
)

type (
	NetworkRuntime struct {
		Running      bool                      `json:"running"`
		StartedAt    time.Time                 `json:"started_at"`
		Sessions     []*Session                `json:"sessions"`
		SessionCount int                       `json:"session_count"`
		Metrics      *metrics.TransportMetrics `json:"metrics"`
	}
	NetworkDetails struct {
		Info    *entity.Network `json:"info"`
		Runtime *NetworkRuntime `json:"runtime"`
	}
	NetworkConfig struct {
		AllowAnonymousDevice bool `json:"allow_anonymous_device"`
		AutoCreateSubDevice  bool `json:"auto_create_sub_device"`
	}
	NetworkLink struct {
		SubDeviceId uint64 `json:"sub_device_id"`
		Key         string `json:"key"`
		Signature   string `json:"signature"`
		Nonce       string `json:"nonce"`
	}
)
