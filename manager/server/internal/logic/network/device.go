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
	g.Log().Infof(ctx, "subdevice: %+v", subDevice)

	network, ok := server.GetNetworkManager().GetNetwork(subDevice.NetworkId)
	if !ok {
		err = errors.New("no network instance running")
		return
	}
	ss.SetNetwork(network)

	// dispatch device
	dev = new(session.Device)
	// todo assign fixed ip
	cidr, err := network.AddressPool().AssignRandom()
	if err != nil {
		return
	}
	dev.Id = subDevice.DeviceId
	dev.CIDR = cidr
	dev.MTU = network.MTU
	network.RegisterSession(cidr, ss)
	return
}
