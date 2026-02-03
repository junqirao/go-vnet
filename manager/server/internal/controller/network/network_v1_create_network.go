package network

import (
	"context"

	"github.com/gogf/gf/v2/util/gconv"

	"go-vnet/manager/server/api/network/v1"
	"go-vnet/manager/server/internal/model/entity"
	"go-vnet/manager/server/internal/service"
)

func (c *ControllerV1) CreateNetwork(ctx context.Context, req *v1.CreateNetworkReq) (res *v1.CreateNetworkRes, err error) {
	if req.Extra == nil {
		req.Extra = make(map[string]any)
	}
	id, err := service.Network().CreateNetwork(ctx, &entity.Network{
		Name:        req.Name,
		Cidr:        req.CIDR,
		Mtu:         1392,
		Description: req.Description,
		Extra:       gconv.String(req.Extra),
	})
	if err != nil {
		return
	}
	res = &v1.CreateNetworkRes{Id: id}
	return
}
