package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/manager/server/internal/model"
)

type GetNetworkDetailsReq struct {
	g.Meta    `path:"/network/details" tags:"Network" method:"get" summary:"Get network details"`
	NetworkId string `json:"network_id"`
}

type GetNetworkDetailsRes model.NetworkDetails
