package network

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/util/gconv"

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

	if lastSessionId := gconv.String(payload["session"]); lastSessionId != "" {
		last, ok := network.SessionById(lastSessionId)
		if ok {
			// stop last session, use in reconnect case
			g.Log().Infof(ctx, "stop last session: %s", lastSessionId)
			last.Stop()
			last.Release(errors.New("new session login"))
		}
	}

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
		P2P:      payload["p2p"],
	}

	bw, err := service.Quota().GetById(ctx, subDevice.BandwidthId)
	if err != nil {
		err = fmt.Errorf("get bandwidth quota failed: %w", err)
		return
	}

	dt, err := service.Quota().GetById(ctx, subDevice.QuotaId)
	if err != nil {
		err = fmt.Errorf("get bandwidth quota failed: %w", err)
		return
	}

	dev.Id = subDevice.DeviceId
	dev.Sid = subDevice.Id
	dev.Name = deviceInfo.Name
	dev.CIDR = cidr
	dev.MTU = network.MTU

	// quota
	dev.BandwidthQuota = &session.Quota{
		Id:     bw.Id,
		Unit:   bw.Unit,
		Value:  bw.Value,
		Period: bw.Period,
	}
	dev.DataTrafficQuota = &session.Quota{
		Id:     dt.Id,
		Unit:   dt.Unit,
		Value:  dt.Value,
		Period: dt.Period,
	}

	network.RegisterSession(cidr, ss)
	return
}
