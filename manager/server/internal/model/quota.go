package model

import (
	"go-vnet/manager/server/internal/model/entity"
)

type Quota struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Value  int64  `json:"value"`
	Period string `json:"period"`
}

type QuotaDetail struct {
	Quota    *Quota              `json:"quota"`
	Usage    int64               `json:"usage"`
	IsExceed bool                `json:"is_exceed"`
	Flow     []*entity.QuotaFlow `json:"flow"`
}
