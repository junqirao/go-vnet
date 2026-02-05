package network

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/common/session"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func (s *sNetwork) AcquireDevice(ctx context.Context, ss *server.Session, subDeviceId uint64, payload map[string]any) (dev *session.Device, err error) {
	subDevice, err := service.Device().GetSubDeviceById(ctx, subDeviceId)
	if err != nil {
		return
	}

	network, ok := server.GetNetworkManager().GetNetwork(subDevice.NetworkId)
	if !ok {
		err = errors.New("no network instance running")
		return
	}
	ss.SetNetwork(network)

	deviceInfo, err := service.Device().GetDeviceById(ctx, subDevice.DeviceId)
	if err != nil {
		return
	}

	// dispatch device
	dev = new(session.Device)
	setting := &model.SubDeviceSettings{}
	if err = json.Unmarshal([]byte(subDevice.Settings), setting); err != nil {
		err = response.DefaultFailure().WithDetail("invalid sub device settings")
		return
	}
	var cidr string
	if setting.FixedIP > 0 {
		g.Log().Infof(ctx, "%s assign by offset: %d", ss.SessionId, setting.FixedIP)
		cidr, err = network.AddressPool().AssignByOffset(uint32(setting.FixedIP))
	} else {
		cidr, err = network.AddressPool().AssignRandom()
	}
	if err != nil {
		return
	}

	ss.ClientInfo.Hostname = payload["hostname"]

	dev.Id = subDevice.DeviceId
	dev.Name = deviceInfo.Name
	dev.CIDR = cidr
	dev.MTU = network.MTU
	network.RegisterSession(cidr, ss)
	return
}
