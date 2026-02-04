package network

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/encoding/gbase64"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

	"go-vnet/common/session"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func (s *sNetwork) AcquireDevice(ctx context.Context, n *server.Network, payload map[string]any) (dev *session.Device, err error) {
	g.Log().Infof(ctx, "AcquireDevice: %+v", payload)
	// pattern:
	// sub_device_id,key,signature,nonce
	linkStr := gconv.String(payload["link"])
	if linkStr == "" {
		// err = response.CodeInvalidParameter.WithDetail("connect is empty")
		return
	}

	bs, err := gbase64.DecodeString(linkStr)
	if err != nil {
		return
	}

	link := new(model.NetworkLink)
	if err = json.Unmarshal(bs, &link); err != nil {
		return
	}

	subDevice, err := service.Device().GetSubDeviceById(ctx, link.SubDeviceId)
	if err != nil {
		return
	}
	g.Log().Infof(ctx, "subdevice: %+v", subDevice)

	// todo
	return
}
