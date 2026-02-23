package main

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"

	"go-vnet/common/config"
	web "go-vnet/manager/server"
	"go-vnet/vnet/server"
)

func main() {
	ctx := gctx.GetInitCtx()
	cfg, err := loadConfigFromFile(ctx)
	if err != nil {
		g.Log().Errorf(ctx, "load server config error: %s", err.Error())
		return
	}

	// run server
	s := server.NewServer(cfg)
	go func() {
		err := s.Serve(ctx)
		if err != nil {
			panic(err)
		}
	}()
	// register manager server
	s.BindManager(web.ManagerServer())

	// start all networks
	web.StartAllNetworks()
	// run web server
	web.RunServer()
}

func loadConfigFromFile(ctx context.Context, opts ...server.ConfigOption) (cfg *server.Config, err error) {
	tmp := server.NewConfig()
	v, err := g.Cfg().Get(ctx, "vnet")
	if err != nil {
		g.Log().Errorf(ctx, "load server config error: %s", err.Error())
		return
	}
	cfg = server.NewConfig(opts...)
	if err = v.Struct(tmp); err != nil {
		return
	}
	cfg.Servers = tmp.Servers
	cfg.P2P = tmp.P2P
	for _, sc := range cfg.Servers {
		sc.MappedConfig = config.NewMappedConfig()
	}
	return
}
