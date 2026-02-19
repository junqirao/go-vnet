package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/vnet/client"
)

type GetRuntimeInfoReq struct {
	g.Meta `path:"/client/runtime" tags:"Client" method:"get" summary:"Get Runtime Info"`
}

type GetRuntimeInfoRes client.RuntimeInfo
