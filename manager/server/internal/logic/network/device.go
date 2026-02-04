package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gogf/gf/v2/encoding/gbase64"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"go-vnet/common/session"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func (s *sNetwork) AcquireDevice(ctx context.Context, ss *server.Session, payload map[string]any) (dev *session.Device, err error) {
	linkStr := gconv.String(payload["link"])
	if linkStr == "" {
		// err = response.CodeInvalidParameter.WithDetail("connect is empty")
		return
	}

	bs, err := gbase64.DecodeString(linkStr)
	if err != nil {
		return
	}

	link := new(server.NetworkLink)
	if err = json.Unmarshal(bs, &link); err != nil {
		return
	}

	subDevice, err := service.Device().GetSubDeviceById(ctx, link.SubDeviceId)
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

	_, err = service.Device().VerifyById(ctx, subDevice.DeviceId, link.Key, link.Nonce, link.Signature)
	if err != nil {
		err = fmt.Errorf("verify device failed: %w", err)
		return
	}

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
