package main

import (
	"context"
	"fmt"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/vnet/server"
	"go-vnet/vnet/server/network"
)

func main() {
	// create test network
	n, err := network.NewNetwork(&network.Config{
		ID:         "test",
		CIDR:       "192.168.98.0/24",
		RouterData: nil,
		MTU:        1392,
	})
	if err != nil {
		panic(err)
	}

	// register network
	network.GetManager().RegisterNetwork(n)

	l := logger.NewStdLogger(nil, "transport_server")
	cfg := server.NewConfig(
		server.WithLogger(l),
		server.WithAuthConfig(auth.Config{
			Type:     auth.TypeSimplePassword,
			Password: "",
		}),
		server.WithAuthenticationChainFunc(func(ctx context.Context, request map[string]any, resp map[string]any) (err error) {
			fmt.Println("------------")
			fmt.Println("Auth Chain Func")
			fmt.Printf("%+v\n", request)
			fmt.Println("------------")
			return nil
		}),
	)
	cfg.Servers = append(cfg.Servers, &server.TransportConfig{
		MappedConfig: config.NewMappedConfig(),
		Port:         9800,
		Address:      "0.0.0.0",
		Type:         server.TypeQuic,
	})
	s := server.NewServer(cfg)
	err = s.Serve(context.Background())
	if err != nil {
		panic(err)
	}
}
