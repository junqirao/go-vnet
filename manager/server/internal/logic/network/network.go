package network

import (
	"context"
	"fmt"

	"github.com/junqirao/gocomponents/response"

	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/service"
	"go-vnet/vnet/server"
)

func init() {
	service.RegisterNetwork(new(sNetwork))
}

type sNetwork struct {
}

func (s *sNetwork) GetNetworkDetails(_ context.Context, networkId string) (details *model.NetworkDetails, err error) {
	n, ok := server.GetNetworkManager().GetNetwork(networkId)
	if !ok {
		err = response.CodeNotFound.WithDetail(fmt.Sprintf("network %s not found", networkId))
		return
	}
	details = &model.NetworkDetails{
		Network: &model.Network{
			Config:  n.NetworkConfig,
			Metrics: n.Metrics,
		},
		Sessions: s.listSession(n),
	}
	details.SessionCount = len(details.Sessions)
	return
}
