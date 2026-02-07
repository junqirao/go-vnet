package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/manager/server/internal/controller/middleware"
	"go-vnet/manager/server/internal/model"
)

type GetNetworkDetailsReq struct {
	g.Meta `path:"/network/:network_id" tags:"Network" method:"get" summary:"Get network details"`
	middleware.RequiredAuthHeader

	NetworkId string `json:"network_id" v:"required" in:"path"`
}

type GetNetworkDetailsRes model.NetworkDetails

type CreateNetworkReq struct {
	g.Meta `path:"/network" tags:"Network" method:"post" summary:"Create network"`
	middleware.RequiredAuthHeader

	Name        string         `json:"name"`
	CIDR        string         `json:"cidr"`
	Description string         `json:"description"`
	Extra       map[string]any `json:"extra"`
}

type CreateNetworkRes struct {
	Id string `json:"id"`
}

type StopNetworkReq struct {
	g.Meta `path:"/network/:network_id/stop" tags:"Network" method:"post" summary:"Stop network"`
	middleware.RequiredAuthHeader

	NetworkId string `json:"network_id" v:"required" in:"path"`
}

type StopNetworkRes struct{}

type StartNetworkReq struct {
	g.Meta `path:"/network/:network_id/start" tags:"Network" method:"post" summary:"Start network"`
	middleware.RequiredAuthHeader

	NetworkId string `json:"network_id" v:"required" in:"path"`
}

type StartNetworkRes struct {
}

type RemoveNetworkReq struct {
	g.Meta `path:"/network/:network_id" tags:"Network" method:"delete" summary:"Remove network"`
	middleware.RequiredAuthHeader

	NetworkId string `json:"network_id" v:"required" in:"path"`
}

type RemoveNetworkRes struct {
}

type UpdateNetworkReq struct {
	g.Meta `path:"/network/:network_id" tags:"Network" method:"post" summary:"Update network"`
	middleware.RequiredAuthHeader

	NetworkId string         `json:"network_id" v:"required" in:"path"`
	Fields    map[string]any `json:"fields"`
}

type UpdateNetworkRes struct{}

type SetNetworkQuotaReq struct {
	g.Meta `path:"/network/:network_id/quota" tags:"Network" method:"post" summary:"Set network quota"`
	middleware.RequiredAuthHeader

	NetworkId string `json:"network_id" v:"required" in:"path"`
	Quota     int    `json:"quota" v:"required"`
}

type SetNetworkQuotaRes struct{}
