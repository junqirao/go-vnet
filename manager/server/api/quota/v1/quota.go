package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/manager/server/internal/controller/middleware"
	"go-vnet/manager/server/internal/model"
	"go-vnet/manager/server/internal/model/entity"
)

type GetQuotaReq struct {
	g.Meta `path:"/quota/:id" tags:"Quota" method:"get" summary:"Get quota by id"`
	middleware.RequiredAuthHeader

	Id int `json:"id" v:"required" in:"path"`
}

type GetQuotaRes model.Quota

type ListQuotaReq struct {
	g.Meta `path:"/quota" tags:"Quota" method:"get" summary:"List all quotas"`
	middleware.RequiredAuthHeader
}

type ListQuotaRes struct {
	Quotas []*entity.Quota `json:"quotas"`
}

type CreateQuotaReq struct {
	g.Meta `path:"/quota" tags:"Quota" method:"post" summary:"Create quota"`
	middleware.RequiredAuthHeader

	Name   string `json:"name" v:"required"`
	Type   string `json:"type" v:"required"`
	Value  int    `json:"value" v:"required"`
	Period string `json:"period"`
}

type CreateQuotaRes struct{}

type UpdateQuotaReq struct {
	g.Meta `path:"/quota/:id" tags:"Quota" method:"post" summary:"Update quota"`
	middleware.RequiredAuthHeader

	Id     int            `json:"id" v:"required" in:"path"`
	Fields map[string]any `json:"fields" v:"required"`
}

type UpdateQuotaRes struct{}

type DeleteQuotaReq struct {
	g.Meta `path:"/quota/:id" tags:"Quota" method:"delete" summary:"Delete quota"`
	middleware.RequiredAuthHeader

	Id int `json:"id" v:"required" in:"path"`
}

type DeleteQuotaRes struct{}
