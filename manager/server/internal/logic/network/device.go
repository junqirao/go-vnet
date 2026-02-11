package network

import (
	"context"
	"errors"

	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/common/session"
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
	var cidr string
	if subDevice.Settings.FixedIP > 0 {
		g.Log().Infof(ctx, "%s assign by offset: %d", ss.SessionId, subDevice.Settings.FixedIP)
		cidr, err = network.AddressPool().AssignByOffset(uint32(subDevice.Settings.FixedIP))
	} else {
		cidr, err = network.AddressPool().AssignRandom()
	}
	if err != nil {
		return
	}

	ss.ClientInfo = server.ClientInfo{
		Hostname: payload["hostname"],
		Encrypt:  payload["encrypt"],
		Compress: payload["compress"],
	}

	dev.Id = subDevice.DeviceId
	dev.Name = deviceInfo.Name
	dev.CIDR = cidr
	dev.MTU = network.MTU
	dev.Quota = subDevice.QuotaId
	dev.Bandwidth = subDevice.BandwidthId
	network.RegisterSession(cidr, ss)
	return
}
