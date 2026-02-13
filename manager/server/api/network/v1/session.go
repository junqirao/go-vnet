package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/manager/server/internal/model"
)

type ListSessionReq struct {
	g.Meta    `path:"/network/session/list" tags:"Session" method:"get" summary:"List all sessions"`
	NetworkId string `json:"network_id"`
}

type ListSessionRes struct {
	Sessions []*model.Session `json:"sessions"`
}
