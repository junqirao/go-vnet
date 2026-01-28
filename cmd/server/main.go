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
	cfg.Servers = append(cfg.Servers,
		&server.TransportConfig{
			MappedConfig: config.NewMappedConfig(),
			Name:         "quic_test",
			Port:         9800,
			Address:      "0.0.0.0",
			Type:         server.TypeQuic,
		},
		&server.TransportConfig{
			MappedConfig: config.NewMappedConfig(),
			Name:         "tcp_test",
			Port:         9801,
			Address:      "0.0.0.0",
			Type:         server.TypeTCP,
		},
	)
	cfg.RelayServer = append(cfg.RelayServer,
		&server.RelayConfig{
			IP:        "0.0.0.0",
			Transport: "udp",
			Port:      9900,
			Version:   "quic-v1",
		},
		&server.RelayConfig{
			IP:        "0.0.0.0",
			Transport: "tcp",
			Port:      9901,
		},
	)
	s := server.NewServer(cfg)
	err = s.Serve(context.Background())
	if err != nil {
		panic(err)
	}
}
