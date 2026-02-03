package network

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/common/session"
	"go-vnet/manager/server/internal/dao"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func init() {
	service.RegisterNetwork(new(sNetwork))
}

type sNetwork struct {
}

func (s *sNetwork) GetNetworkDetails(ctx context.Context, networkId string) (details *model.NetworkDetails, err error) {
	info, err := s.getNetworkById(ctx, networkId)
	if err != nil {
		return
	}
	details = &model.NetworkDetails{
		Info:    info,
		Runtime: &model.NetworkRuntime{},
	}
	n, ok := server.GetNetworkManager().GetNetwork(networkId)
	if ok {
		sessions := s.listSession(n)
		details.Runtime = &model.NetworkRuntime{
			Running:      true,
			StartedAt:    n.StartedAt,
			Sessions:     sessions,
			SessionCount: len(sessions),
			Metrics:      n.Metrics,
		}
	}
	return
}

func (s *sNetwork) CreateNetwork(ctx context.Context, network *entity.Network) (id string, err error) {
	id = uuid.NewString()
	network.Id = id
	_, err = dao.Network.Ctx(ctx).Insert(network)
	if err != nil {
		return
	}
	n, err := server.NewNetwork(&server.NetworkConfig{
		ID:              id,
		CIDR:            network.Cidr,
		MTU:             network.Mtu,
		AllocDeviceFunc: s.AcquireDevice,
	})
	if err != nil {
		return
	}
	server.GetNetworkManager().RegisterNetwork(n)
	return
}

func (s *sNetwork) StopNetwork(ctx context.Context, networkId string) (err error) {
	mgr := server.GetNetworkManager()
	network, ok := mgr.GetNetwork(networkId)
	if !ok {
		err = response.CodeBadGateway.WithDetail(fmt.Sprintf("instance not exist or not running: %s", networkId))
		return
	}

	network.Stop()
	mgr.RemoveNetwork(networkId)
	return
}

func (s *sNetwork) StartNetwork(ctx context.Context, networkId string) (err error) {
	_, ok := server.GetNetworkManager().GetNetwork(networkId)
	if ok {
		err = response.CodeBadGateway.WithDetail(fmt.Sprintf("instance already started: %s", networkId))
		return
	}

	en, err := s.getNetworkById(ctx, networkId)
	if err != nil {
		return
	}
	n, err := server.NewNetwork(&server.NetworkConfig{
		ID:              en.Id,
		CIDR:            en.Cidr,
		MTU:             en.Mtu,
		AllocDeviceFunc: s.AcquireDevice,
	})
	if err != nil {
		return
	}
	server.GetNetworkManager().RegisterNetwork(n)
	return
}

func (s *sNetwork) getNetworkById(ctx context.Context, networkId string) (en *entity.Network, err error) {
	v, err := dao.Network.Ctx(ctx).One(dao.Network.Columns().Id, networkId)
	if err != nil {
		err = response.CodeFromHttpStatus(http.StatusInternalServerError).WithDetail(fmt.Sprintf("database error: %s", err.Error()))
		return
	}
	en = &entity.Network{}
	if err = v.Struct(en); err != nil {
		if errors.Is(gerror.Cause(err), sql.ErrNoRows) {
			err = response.CodeNotFound.WithDetail(networkId)
		} else {
			err = response.CodeFromHttpStatus(http.StatusInternalServerError).WithDetail(err.Error())
		}
	}
	return
}

func (s *sNetwork) DeleteNetwork(ctx context.Context, networkId string) (err error) {
	// check
	_, err = s.getNetworkById(ctx, networkId)
	if err != nil {
		return
	}
	// stop
	mgr := server.GetNetworkManager()
	n, ok := mgr.GetNetwork(networkId)
	if ok {
		n.Stop()
		mgr.RemoveNetwork(networkId)
	}
	// delete
	_, err = dao.Network.Ctx(ctx).Delete(dao.Network.Columns().Id, networkId)
	if err != nil {
		return
	}
	return
}

func (s *sNetwork) UpdateNetwork(ctx context.Context, networkId string, fields map[string]any) (err error) {
	// must stop all sessions
	_, err = s.getNetworkById(ctx, networkId)
	if err != nil {
		return
	}
	mgr := server.GetNetworkManager()
	n, ok := mgr.GetNetwork(networkId)
	if ok {
		n.Stop()
		mgr.RemoveNetwork(networkId)
	}

	updateMap := g.Map{}
	canUpdateField := []string{
		dao.Network.Columns().Cidr,
		dao.Network.Columns().Name,
	}
	for _, f := range canUpdateField {
		if v, ok := fields[f]; ok && v != "" {
			updateMap[f] = v
		}
	}
	// description can be empty
	if desc, ok := fields[dao.Network.Columns().Description]; ok {
		updateMap[dao.Network.Columns().Description] = desc
	}
	_, err = dao.Network.Ctx(ctx).Where(dao.Network.Columns().Id, networkId).Update(updateMap)
	return
}

func (s *sNetwork) ListNetworkInfos(ctx context.Context) (ns []*entity.Network, err error) {
	result, err := dao.Network.Ctx(ctx).All()
	if err != nil {
		err = response.CodeFromHttpStatus(http.StatusInternalServerError).WithDetail(err.Error())
		return
	}
	ns = make([]*entity.Network, 0)
	for _, record := range result {
		n := &entity.Network{}
		_ = record.Struct(&n)
		ns = append(ns, n)
	}
	return
}

func (s *sNetwork) AcquireDevice(ctx context.Context, n *server.Network, payload map[string]any) (dev *session.Device, err error) {
	// todo
	g.Log().Infof(ctx, "AcquireDevice: %+v", payload)
	return
}
