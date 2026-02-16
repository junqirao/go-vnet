package cmd

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/manager/server/internal/controller/device"
	"go-vnet/manager/server/internal/controller/generic"
	"go-vnet/manager/server/internal/controller/middleware"
	"go-vnet/manager/server/internal/controller/network"
	"go-vnet/manager/server/internal/controller/quota"
)

var (
	WebServer = gcmd.Command{
		Name:  "webserver",
		Usage: "webserver",
		Brief: "start go vnet server web manager server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			s := g.Server()
			s.Group("/", func(group *ghttp.RouterGroup) {
				group.Middleware(middleware.CheckSignature, ghttp.MiddlewareCORS, response.Middleware)
				group.Group("/v1", func(group *ghttp.RouterGroup) {
					group.Bind(
						network.NewV1(),
						device.NewV1(),
						quota.NewV1(),
						generic.NewV1(),
					)
				})
			})
			s.Run()
			return nil
		},
	}
)
