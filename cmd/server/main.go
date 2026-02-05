package main

import (
	"context"

	"go-vnet/common/config"
	"go-vnet/common/logger"
	web "go-vnet/manager/server"
	"go-vnet/vnet/server"
)

func main() {
	// create test network
	// n, err := server.NewNetwork(&server.NetworkConfig{
	// 	ID:         "test",
	// 	CIDR:       "192.168.98.0/24",
	// 	RouterData: nil,
	// 	MTU:        1392,
	// })
	// if err != nil {
	// 	panic(err)
	// }

	// register network
	// server.GetNetworkManager().RegisterNetwork(n)

	l := logger.NewStdLogger(nil, "transport_server")
	cfg := server.NewConfig(
		server.WithLogger(l),
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
	cfg.P2P = &server.P2PConfig{
		Addresses: []server.SignalingServerAddress{
			{
				IP:        "0.0.0.0",
				Transport: "tcp",
				Port:      9901,
			},
		},
	}

	// run server
	s := server.NewServer(cfg)
	go func() {
		err := s.Serve(context.Background())
		if err != nil {
			panic(err)
		}
	}()
	// register manager server
	s.RegisterManager(web.ManagerServer())

	// start all networks
	web.StartAllNetworks()
	// run web server
	web.RunServer()
}
