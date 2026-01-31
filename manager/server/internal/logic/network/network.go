package network

import (
	"go-vnet/manager/server/internal/service"
)

func init() {
	service.RegisterNetwork(new(sNetwork))
}

type sNetwork struct {
}
