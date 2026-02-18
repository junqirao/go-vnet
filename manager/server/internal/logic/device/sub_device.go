package device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/common/quota"
	"go-vnet/manager/server/internal/dao"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func (d *sDevice) CreateSubDevice(ctx context.Context,
	deviceId, networkId string, quota int, bandwidthQuota int,
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
		Bandwidth: bandwidthQuota,
		Settings:  gconv.String(settings),
	})
	return
}

func (d *sDevice) GetSubDeviceWithUsageById(ctx context.Context, id uint64) (sub *model.SubDevice, err error) {
	dev := new(entity.NetworkDevice)
	err = dao.NetworkDevice.Ctx(ctx).Where(dao.NetworkDevice.Columns().Id, id).Scan(dev)
	if err != nil {
		return
	}
	return d.fillSubDeviceQuotaDetails(ctx, d.buildSubDeviceBrief(ctx, dev))
}

func (d *sDevice) GetSubDeviceById(ctx context.Context, id uint64) (sub *model.SubDeviceBrief, err error) {
	dev := new(entity.NetworkDevice)
	if err = dao.NetworkDevice.Ctx(ctx).Where(dao.NetworkDevice.Columns().Id, id).Scan(dev); err != nil {
		return
	}
	sub = d.buildSubDeviceBrief(ctx, dev)
	return
}

func (d *sDevice) GetSubDeviceByIds(ctx context.Context, ids []uint64) (list []*model.SubDeviceBrief, err error) {
	if len(ids) == 0 {
		return []*model.SubDeviceBrief{}, nil
	}
	results, err := dao.NetworkDevice.Ctx(ctx).
		WhereIn(dao.NetworkDevice.Columns().Id, ids).
		All()
	if err != nil {
		return
	}
	list = make([]*model.SubDeviceBrief, 0, len(results))
	for _, result := range results {
		dev := new(entity.NetworkDevice)
		if err = result.Struct(dev); err != nil {
			return
		}
		list = append(list, d.buildSubDeviceBrief(ctx, dev))
	}
	return
}

func (d *sDevice) GetSubDevicesWithUsageByIds(ctx context.Context, ids []uint64) (list []*model.SubDevice, err error) {
	if len(ids) == 0 {
		return []*model.SubDevice{}, nil
	}
	results, err := dao.NetworkDevice.Ctx(ctx).
		WhereIn(dao.NetworkDevice.Columns().Id, ids).
		All()
	if err != nil {
		return
	}
	list = make([]*model.SubDevice, 0, len(results))
	for _, result := range results {
		dev := new(entity.NetworkDevice)
		if err = result.Struct(dev); err != nil {
			return
		}
		brief := d.buildSubDeviceBrief(ctx, dev)
		sub, err := d.fillSubDeviceQuotaDetails(ctx, brief)
		if err != nil {
			return nil, err
		}
		list = append(list, sub)
	}
	return
}

func (d *sDevice) buildSubDeviceBrief(ctx context.Context, dev *entity.NetworkDevice) *model.SubDeviceBrief {
	sub := &model.SubDeviceBrief{
		Id:          dev.Id,
		QuotaId:     dev.Quota,
		DeviceId:    dev.DeviceId,
		BandwidthId: dev.Bandwidth,
		NetworkId:   dev.NetworkId,
		Settings:    &model.SubDeviceSettings{},
	}
	if err := json.Unmarshal([]byte(dev.Settings), sub.Settings); err != nil {
		g.Log().Errorf(ctx, "unmarshal sub device settings error: %v", err)
	}
	return sub
}

func (d *sDevice) fillSubDeviceQuotaDetails(ctx context.Context, brief *model.SubDeviceBrief) (sub *model.SubDevice, err error) {
	sub = &model.SubDevice{
		SubDeviceBrief: brief,
	}
	sub.Quota, err = service.Quota().GetQuotaDetails(ctx, sub.QuotaId, sub.DeviceId, quota.TargetTypeDeviceTraffic, true)
	if err != nil {
		return
	}
	sub.Bandwidth, err = service.Quota().GetQuotaDetails(ctx, sub.BandwidthId, sub.DeviceId, quota.TargetTypeDeviceBandwidth, false)
	if err != nil {
		return
	}
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

func (d *sDevice) SetSubDeviceBandwidthQuota(ctx context.Context, subDeviceId uint64, bandwidthQuota int) (err error) {
	// check sub device exists
	_, err = d.GetSubDeviceById(ctx, subDeviceId)
	if err != nil {
		return
	}

	// update bandwidth quota
	_, err = dao.NetworkDevice.Ctx(ctx).Where(dao.NetworkDevice.Columns().Id, subDeviceId).Update(g.Map{
		dao.NetworkDevice.Columns().Bandwidth: bandwidthQuota,
	})
	return
}

func (d *sDevice) GetSubDeviceListByDeviceId(ctx context.Context, deviceId string) (list []*model.SubDevice, err error) {
	// check device exists
	_, err = service.Device().GetDeviceById(ctx, deviceId)
	if err != nil {
		return
	}

	results, err := dao.NetworkDevice.Ctx(ctx).
		Where(dao.NetworkDevice.Columns().DeviceId, deviceId).
		All()
	if err != nil {
		return
	}

	list = make([]*model.SubDevice, 0, len(results))
	for _, result := range results {
		dev := new(entity.NetworkDevice)
		if err = result.Struct(&dev); err != nil {
			return
		}
		sub, err := d.fillSubDeviceQuotaDetails(ctx, d.buildSubDeviceBrief(ctx, dev))
		if err != nil {
			return nil, err
		}
		list = append(list, sub)
	}
	return
}

func (d *sDevice) DeleteSubDeviceById(ctx context.Context, id uint64) (err error) {
	sub, err := d.GetSubDeviceById(ctx, id)
	if err != nil {
		return
	}
	network, ok := server.GetNetworkManager().GetNetwork(sub.NetworkId)
	if ok {
		for _, session := range network.ListSessions() {
			if session.Session.DispatchedDevice.Sid == sub.Id {
				err = response.CodeInvalidParameter.WithDetail(fmt.Sprintf("sub device is in use: %s", session.Session.SessionId))
				return
			}
		}
	}
	_, err = dao.NetworkDevice.Ctx(ctx).Where(dao.NetworkDevice.Columns().Id, id).Delete()
	return
}
