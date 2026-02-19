package cmd

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/junqirao/gocomponents/response"

	_ "go-vnet/manager/client/internal/logic"
	_ "go-vnet/manager/client/internal/packed"

	"go-vnet/manager/client/internal/controller/client"
	"go-vnet/manager/client/internal/controller/middieware"
)

var (
	WebServer = gcmd.Command{
		Name:  "webserver",
		Usage: "webserver",
		Brief: "start go vnet client web manager server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			s := g.Server()

			if g.Cfg().MustGet(ctx, "debug").Bool() {
				s.SetDumpRouterMap(true)
				s.SetOpenApiPath("/api.json")
				s.SetSwaggerPath("/swagger")
				go func() {
					g.Log().Infof(ctx, "start pprof at :6060")
					g.Log().Info(ctx, http.ListenAndServe("0.0.0.0:6060", nil))
				}()
				g.Log().Infof(ctx, "debug mode enabled")
			} else {
				s.SetDumpRouterMap(false)
				s.SetOpenApiPath("")
				s.SetSwaggerPath("")
			}

			s.SetAddr(fmt.Sprintf("%s:%d",
				g.Cfg().MustGet(ctx, "manager.listen").String(),
				g.Cfg().MustGet(ctx, "manager.port").Int()))
			s.Group("/", func(group *ghttp.RouterGroup) {
				group.Middleware(ghttp.MiddlewareCORS, response.Middleware)
				if allows := g.Cfg().MustGet(ctx, "manager.allowed_ip", []string{}).Strings(); len(allows) > 0 {
					g.Log().Infof(ctx, "manager server allowed ip: %v", allows)
					group.Middleware(middieware.WhiteList(allows))
				}
				group.Bind(
					client.NewV1(),
				)
			})
			s.Run()
			return nil
		},
	}
)
