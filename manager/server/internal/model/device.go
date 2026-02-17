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
	DeviceInfo struct {
		Id        string      `json:"id"`
		Name      string      `json:"name"`
		CreatedAt *gtime.Time `json:"created_at"`
	}
	SubDeviceBrief struct {
		Id          int                `json:"id"`
		QuotaId     int                `json:"quota_id"`
		BandwidthId int                `json:"bandwidth_id"`
		DeviceId    string             `json:"device_id"`
		NetworkId   string             `json:"network_id"`
		Settings    *SubDeviceSettings `json:"settings"`
	}
	SubDevice struct {
		*SubDeviceBrief
		Quota     *QuotaDetail `json:"quota"`
		Bandwidth *QuotaDetail `json:"bandwidth"`
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
