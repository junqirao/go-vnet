package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/manager/server/internal/controller/middleware"
	"go-vnet/manager/server/internal/model"
)

type GetGenericInfoReq struct {
	g.Meta `path:"/generic" tags:"Generic" method:"get" summary:"Get Generic Info"`
	middleware.RequiredAuthHeader

	Period string `json:"period"`
}

type GetGenericInfoRes model.GenericInfo
