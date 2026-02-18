// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package device

import (
	"context"

	"go-vnet/manager/server/api/device/v1"
)

type IDeviceV1 interface {
	CreateDevice(ctx context.Context, req *v1.CreateDeviceReq) (res *v1.CreateDeviceRes, err error)
	CreateSubDevice(ctx context.Context, req *v1.CreateSubDeviceReq) (res *v1.CreateSubDeviceRes, err error)
	GetDevice(ctx context.Context, req *v1.GetDeviceReq) (res *v1.GetDeviceRes, err error)
	SetSubDeviceQuota(ctx context.Context, req *v1.SetSubDeviceQuotaReq) (res *v1.SetSubDeviceQuotaRes, err error)
	SetSubDeviceBandwidthQuota(ctx context.Context, req *v1.SetSubDeviceBandwidthQuotaReq) (res *v1.SetSubDeviceBandwidthQuotaRes, err error)
	GetDeviceList(ctx context.Context, req *v1.GetDeviceListReq) (res *v1.GetDeviceListRes, err error)
	GetSubDeviceListByDeviceId(ctx context.Context, req *v1.GetSubDeviceListByDeviceIdReq) (res *v1.GetSubDeviceListByDeviceIdRes, err error)
	DeleteDevice(ctx context.Context, req *v1.DeleteDeviceReq) (res *v1.DeleteDeviceRes, err error)
	DeleteSubDevice(ctx context.Context, req *v1.DeleteSubDeviceReq) (res *v1.DeleteSubDeviceRes, err error)
}
