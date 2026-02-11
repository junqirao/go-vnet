package device

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/common/quota"
	"go-vnet/manager/server/internal/dao"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
)

func (d *sDevice) CreateSubDevice(ctx context.Context,
	deviceId, networkId string, quota int,
	settings *model.SubDeviceSettings) (err error) {
	network, err := service.Network().GetNetworkById(ctx, networkId)
	if err != nil {
		return
	}
	device, err := service.Device().GetDeviceById(ctx, deviceId)
	if err != nil {
		return
	}
	exist, err := dao.NetworkDevice.Ctx(ctx).Where(g.Map{
		dao.NetworkDevice.Columns().NetworkId: network.Id,
		dao.NetworkDevice.Columns().DeviceId:  device.Id,
	}).Exist()
	if err != nil {
		return
	}
	if exist {
		err = response.CodeConflict.WithDetail("sub device already exists")
		return
	}
	if settings == nil {
		settings = model.DefaultSubDeviceSettings
	}
	_, err = dao.NetworkDevice.Ctx(ctx).Insert(&entity.NetworkDevice{
		DeviceId:  device.Id,
		NetworkId: network.Id,
		Quota:     quota,
		Settings:  gconv.String(settings),
	})
	return
}

func (d *sDevice) GetSubDeviceById(ctx context.Context, id uint64) (sub *model.SubDevice, err error) {
	dev := new(entity.NetworkDevice)
	err = dao.NetworkDevice.Ctx(ctx).Where(dao.NetworkDevice.Columns().Id, id).Scan(dev)
	if err != nil {
		return
	}
	sub = &model.SubDevice{
		Id:        dev.Id,
		QuotaId:   dev.Quota,
		DeviceId:  dev.DeviceId,
		NetworkId: dev.NetworkId,
		Settings:  &model.SubDeviceSettings{},
	}
	if err = json.Unmarshal([]byte(dev.Settings), sub.Settings); err != nil {
		return
	}
	sub.Quota, err = service.Quota().GetQuotaDetails(ctx, sub.QuotaId, sub.DeviceId, quota.TargetTypeDevice)
	return
}

func (d *sDevice) SetSubDeviceQuota(ctx context.Context, subDeviceId uint64, quota int) (err error) {
	// check sub device exists
	_, err = d.GetSubDeviceById(ctx, subDeviceId)
	if err != nil {
		return
	}

	// update quota
	_, err = dao.NetworkDevice.Ctx(ctx).Where(dao.NetworkDevice.Columns().Id, subDeviceId).Update(g.Map{
		dao.NetworkDevice.Columns().Quota: quota,
	})
	return
}
