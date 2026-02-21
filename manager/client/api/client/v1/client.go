package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"go-vnet/vnet/client"
)

type GetRuntimeInfoReq struct {
	g.Meta   `path:"/client/runtime" tags:"Client" method:"get" summary:"Get Runtime Info"`
	Duration string `json:"duration"`
}

type GetRuntimeInfoRes client.RuntimeInfo

type StopReq struct {
	g.Meta `path:"/client/stop" tags:"Client" method:"post" summary:"Stop Client"`
}

type StopRes struct{}

type ResumeReq struct {
	g.Meta `path:"/client/resume" tags:"Client" method:"post" summary:"Resume Client"`
}

type ResumeRes struct{}
