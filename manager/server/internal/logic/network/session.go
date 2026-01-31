package network

import (
	"context"
	"fmt"

	"go-vnet/vnet/server"
)

func (s *sNetwork) ListSessionByNetwork(ctx context.Context, networkId string) (sessions []*server.Session, err error) {
	n, ok := server.GetNetworkManager().GetNetwork(networkId)
	if !ok {
		return nil, fmt.Errorf("network %s not found", networkId)
	}
	sessions = n.ListSessions()
	return
}
