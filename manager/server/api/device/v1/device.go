package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/manager/server/internal/controller/middleware"
	"go-vnet/manager/server/internal/model"
)

type CreateDeviceReq struct {
	g.Meta `path:"/device" tags:"Device" method:"post" summary:"Create Device"`
	middleware.RequiredAuthHeader

	Key  string `json:"key" v:"required"`
	Name string `json:"name" v:"required"`
}

type CreateDeviceRes struct {
	Id string `json:"id"`
}

type CreateSubDeviceReq struct {
	g.Meta `path:"/device/:device_id/sub" tags:"Device" method:"post" summary:"Create Sub Device"`
	middleware.RequiredAuthHeader

	DeviceId  string                   `json:"device_id" v:"required" in:"path"`
	NetworkId string                   `json:"network_id" v:"required"`
	Quota     int                      `json:"quota"`
	Settings  *model.SubDeviceSettings `json:"settings"`
}

type CreateSubDeviceRes struct{}

type GetDeviceReq struct {
	g.Meta `path:"/device/:device_id" tags:"Device" method:"get" summary:"Get Device Info"`
	middleware.RequiredAuthHeader

	DeviceId    string `json:"device_id" v:"required" in:"path"`
	SubDeviceId uint64 `json:"sub_device_id"`
}

type GetDeviceRes model.Device

type SetSubDeviceQuotaReq struct {
	g.Meta `path:"/sub/:sub_device_id/quota" tags:"Device" method:"post" summary:"Set sub device quota"`
	middleware.RequiredAuthHeader

	SubDeviceId uint64 `json:"sub_device_id" v:"required" in:"path"`
	Quota       int    `json:"quota" v:"required"`
}

type SetSubDeviceQuotaRes struct{}
