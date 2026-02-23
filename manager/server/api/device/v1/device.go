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

	DeviceId       string                   `json:"device_id" v:"required" in:"path"`
	NetworkId      string                   `json:"network_id" v:"required"`
	Quota          int                      `json:"quota"`
	BandwidthQuota int                      `json:"bandwidth_quota"`
	Settings       *model.SubDeviceSettings `json:"settings"`
}

type CreateSubDeviceRes struct{}

type GetDeviceReq struct {
	g.Meta `path:"/device/:device_id" tags:"Device" method:"get" summary:"Get Device Info"`
	middleware.RequiredAuthHeader

	DeviceId    string `json:"device_id" v:"required" in:"path"`
	SubDeviceId uint64 `json:"sub_device_id"`
}

type GetDeviceRes model.Device

type GetDevicePublicKeyReq struct {
	g.Meta `path:"/device/:device_id/public_key" tags:"Device" method:"get" summary:"GetDevice Public Key"`
	middleware.RequiredAuthHeader

	DeviceId string `json:"device_id" v:"required" in:"path"`
}

type GetDevicePublicKeyRes struct {
	PublicKey string `json:"public_key"`
}

type SetSubDeviceQuotaReq struct {
	g.Meta `path:"/sub/:sub_device_id/quota" tags:"Device" method:"post" summary:"Set sub device quota"`
	middleware.RequiredAuthHeader

	SubDeviceId uint64 `json:"sub_device_id" v:"required" in:"path"`
	Quota       int    `json:"quota" v:"required"`
}

type SetSubDeviceQuotaRes struct{}

type SetSubDeviceBandwidthQuotaReq struct {
	g.Meta `path:"/sub/:sub_device_id/bandwidth" tags:"Device" method:"post" summary:"Set sub device bandwidth quota"`
	middleware.RequiredAuthHeader

	SubDeviceId    uint64 `json:"sub_device_id" v:"required" in:"path"`
	BandwidthQuota int    `json:"bandwidth_quota" v:"required"`
}

type SetSubDeviceBandwidthQuotaRes struct{}

type GetDeviceListReq struct {
	g.Meta `path:"/devices" tags:"Device" method:"get" summary:"Get Device List with Pagination"`
	middleware.RequiredAuthHeader

	Page     int    `json:"page" d:"1" v:"required|min:1"`
	PageSize int    `json:"page_size" d:"10" v:"required|min:1|max:100"`
	Name     string `json:"name"`
}

type GetDeviceListRes struct {
	List  []*model.DeviceInfo `json:"list"`
	Total int                 `json:"total"`
}

type GetSubDeviceListByDeviceIdReq struct {
	g.Meta `path:"/device/:device_id/subs" tags:"Device" method:"get" summary:"Get Sub Device List By Device Id"`
	middleware.RequiredAuthHeader

	DeviceId string `json:"device_id" v:"required" in:"path"`
}

type GetSubDeviceListByDeviceIdRes struct {
	List []*model.SubDevice `json:"list"`
}

type DeleteDeviceReq struct {
	g.Meta `path:"/device/:device_id" tags:"Device" method:"delete" summary:"Delete Device"`
	middleware.RequiredAuthHeader

	DeviceId string `json:"device_id" v:"required" in:"path"`
}

type DeleteDeviceRes struct{}

type DeleteSubDeviceReq struct {
	g.Meta `path:"/sub/:sub_device_id" tags:"Device" method:"delete" summary:"Delete Sub Device"`
	middleware.RequiredAuthHeader

	SubDeviceId uint64 `json:"sub_device_id" v:"required" in:"path"`
}

type DeleteSubDeviceRes struct{}
