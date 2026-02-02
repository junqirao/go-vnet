package network

import (
	"context"
	"fmt"

	"github.com/junqirao/gocomponents/response"

	"go-vnet/manager/server/internal/model"
	"go-vnet/vnet/server"
)

func (s *sNetwork) ListSessionByNetwork(_ context.Context, networkId string) (sessions []*model.Session, err error) {
	n, ok := server.GetNetworkManager().GetNetwork(networkId)
	if !ok {
		err = response.CodeNotFound.WithDetail(fmt.Sprintf("network %s not found", networkId))
		return
	}

	sessions = s.listSession(n)
	return
}

func (s *sNetwork) listSession(n *server.Network) (sessions []*model.Session) {
	for _, ss := range n.ListSessions() {
		sessions = append(sessions, &model.Session{
			SessionId:        ss.SessionId,
			Network:          ss.Network().NetworkConfig.CIDR,
			IP:               ss.IP,
			Type:             ss.Type,
			DispatchedDevice: ss.DispatchedDevice,
			Metrics:          ss.Metrics,
			CreatedAt:        ss.CreatedAt,
		})
	}
	return
}
