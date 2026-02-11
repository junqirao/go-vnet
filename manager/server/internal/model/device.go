package model

import (
	"github.com/gogf/gf/v2/os/gtime"
)

const (
	DeviceModeProxyOnly DeviceMode = 1 << iota
	DeviceModeP2POnly
	DeviceModeMixed
)

var (
	DefaultSubDeviceSettings = &SubDeviceSettings{
		FixedIP:    0,
		DeviceMode: DeviceModeMixed,
	}
)

type (
	Device struct {
		Id        string       `json:"id"`
		Name      string       `json:"name"`
		CreatedAt *gtime.Time  `json:"created_at"`
		Sub       []*SubDevice `json:"sub"`
	}
	SubDevice struct {
		Id        int                `json:"id"`
		QuotaId   int                `json:"quota_id"`
		DeviceId  string             `json:"device_id"`
		NetworkId string             `json:"network_id"`
		Quota     *QuotaDetail       `json:"quota"`
		Settings  *SubDeviceSettings `json:"settings"`
	}
	DeviceMode        int
	SubDeviceSettings struct {
		// FixedIP allocation, records ip position
		// e.g. 32  -> x.x.x.32/24
		// e.g. 262 -> x.x.1.12/16
		FixedIP int `json:"fixed_ip"`
		// DeviceMode supports proxy only, p2p only, mixed
		DeviceMode DeviceMode `json:"device_mode"`
	}
)
