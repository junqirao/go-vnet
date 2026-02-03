package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/manager/server/internal/model"
)

type GetNetworkDetailsReq struct {
	g.Meta    `path:"/network/:network_id" tags:"Network" method:"get" summary:"Get network details"`
	NetworkId string `json:"network_id"`
}

type GetNetworkDetailsRes model.NetworkDetails

type CreateNetworkReq struct {
	g.Meta      `path:"/network" tags:"Network" method:"post" summary:"Create network"`
	Name        string         `json:"name"`
	CIDR        string         `json:"cidr"`
	Description string         `json:"description"`
	Extra       map[string]any `json:"extra"`
}

type CreateNetworkRes struct {
	Id string `json:"id"`
}

type StopNetworkReq struct {
	g.Meta    `path:"/network/:network_id/stop" tags:"Network" method:"post" summary:"Stop network"`
	NetworkId string `json:"network_id"`
}

type StopNetworkRes struct{}

type StartNetworkReq struct {
	g.Meta    `path:"/network/:network_id/start" tags:"Network" method:"post" summary:"Start network"`
	NetworkId string `json:"network_id"`
}

type StartNetworkRes struct {
}

type RemoveNetworkReq struct {
	g.Meta    `path:"/network/:network_id" tags:"Network" method:"delete" summary:"Remove network"`
	NetworkId string `json:"network_id"`
}

type RemoveNetworkRes struct {
}

type UpdateNetworkReq struct {
	g.Meta    `path:"/network/:network_id" tags:"Network" method:"post" summary:"Update network"`
	NetworkId string         `json:"network_id"`
	Fields    map[string]any `json:"fields"`
}

type UpdateNetworkRes struct{}
