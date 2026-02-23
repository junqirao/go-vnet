package cmd

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/junqirao/gocomponents/response"

	"go-vnet/common/tls"
	"go-vnet/manager/server/internal/controller/device"
	"go-vnet/manager/server/internal/controller/generic"
	"go-vnet/manager/server/internal/controller/middleware"
	"go-vnet/manager/server/internal/controller/network"
	"go-vnet/manager/server/internal/controller/quota"
)

const (
	defaultCertFile = "./tmp_server.crt"
	defaultKeyFile  = "./tmp_server.key"
)

var (
	WebServer = gcmd.Command{
		Name:  "webserver",
		Usage: "webserver",
		Brief: "start go vnet server web manager server",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			s := g.Server()

			// 配置HTTPS
			certFile := g.Cfg().MustGet(ctx, "server.cert_file").String()
			keyFile := g.Cfg().MustGet(ctx, "server.key_file").String()

			if certFile != "" && keyFile != "" {
				// 使用配置文件中的证书
				s.EnableHTTPS(certFile, keyFile)
				g.Log().Infof(ctx, "HTTPS enabled with cert: %s, key: %s", certFile, keyFile)
			} else {
				err := tls.GenerateSelfSignedCertToFile(defaultCertFile, defaultKeyFile)
				if err != nil {
					g.Log().Errorf(ctx, "failed to generate self-signed cert: %s", err.Error())
					return err
				}
				s.EnableHTTPS(defaultCertFile, defaultKeyFile)
				g.Log().Infof(ctx, "HTTPS enabled with self-signed cert: %s", certFile)
			}

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
