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

	"go-vnet/common/tls"
	"go-vnet/manager/client/embed"
	_ "go-vnet/manager/client/internal/logic"
	_ "go-vnet/manager/client/internal/packed"

	"go-vnet/manager/client/internal/controller/client"
	"go-vnet/manager/client/internal/controller/middieware"
)

const (
	defaultCertFile = "./tmp_client_manager.crt"
	defaultKeyFile  = "./tmp_client_manager.key"
)

var (
	WebServer = gcmd.Command{
		Name:  "webserver",
		Usage: "webserver",
		Brief: "start go vnet client web manager server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			s := g.Server()

			// HTTPS
			certFile := g.Cfg().MustGet(ctx, "manager.cert_file").String()
			keyFile := g.Cfg().MustGet(ctx, "manager.key_file").String()

			if certFile == "" || keyFile == "" {
				certFile = defaultCertFile
				keyFile = defaultKeyFile
				err := tls.GenerateSelfSignedCertToFile(certFile, keyFile)
				if err != nil {
					g.Log().Errorf(ctx, "failed to generate self-signed cert: %s", err.Error())
					return err
				}
			}
			s.EnableHTTPS(certFile, keyFile)

			s.SetAddr(fmt.Sprintf("%s:%d",
				g.Cfg().MustGet(ctx, "manager.listen").String(),
				g.Cfg().MustGet(ctx, "manager.port").Int()))

			if g.Cfg().MustGet(ctx, "debug").Bool() {
				s.SetDumpRouterMap(true)
				s.SetOpenApiPath("/api.json")
				s.SetSwaggerPath("/swagger")
				go func() {
					g.Log().Infof(ctx, "start pprof at :6060")
					g.Log().Info(ctx, http.ListenAndServe("0.0.0.0:6060", nil))
				}()
				g.Log().SetStack(true)
				g.Log().Infof(ctx, "debug mode enabled")
			} else {
				s.SetDumpRouterMap(false)
				s.SetOpenApiPath("")
				s.SetSwaggerPath("")
				g.Log().SetStack(false)
			}

			// ui
			if g.Cfg().MustGet(ctx, "ui.enabled", true).Bool() {
				uiFS, err := embed.GetUIFS()
				if err != nil {
					g.Log().Errorf(ctx, "get ui fs failed: %v", err)
				} else {
					// /ui 路由组
					s.Group("/ui", func(group *ghttp.RouterGroup) {
						group.ALL("/*", ghttp.WrapH(http.StripPrefix("/ui", http.FileServer(http.FS(uiFS)))))
					})
					g.Log().Infof(ctx, "ui enabled at /ui")

					// 根路径的静态资源重定向到 /ui（支持前端 SPA）
					s.Group("/", func(group *ghttp.RouterGroup) {
						// 处理静态资源请求：assets/, favicon.ico 等
						group.ALL("/assets/*", func(r *ghttp.Request) {
							r.Response.RedirectTo("/ui"+r.URL.Path, http.StatusMovedPermanently)
						})
						group.ALL("/favicon.ico", func(r *ghttp.Request) {
							r.Response.RedirectTo("/ui/favicon.ico", http.StatusMovedPermanently)
						})
						// 根路径重定向到 /ui
						group.GET("/", func(r *ghttp.Request) {
							r.Response.RedirectTo("/ui/", http.StatusMovedPermanently)
						})
					})
				}
			}

			s.Group("/", func(group *ghttp.RouterGroup) {
				group.Middleware(ghttp.MiddlewareCORS, ghttp.MiddlewareGzip, response.Middleware)
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
