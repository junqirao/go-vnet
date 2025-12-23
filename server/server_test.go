package server

import (
	"context"
	"testing"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/logger"
	"go-vnet/server/network"
	"go-vnet/server/transport"
)

func TestServer_Run(t *testing.T) {
	// create network
	n, err := network.NewNetwork(&network.Config{
		ID:         "test",
		CIDR:       "192.168.98.0/24",
		RouterData: nil,
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	// register network
	network.GetManager().RegisterNetwork(n)

	// create transport
	l := logger.NewStdLogger(nil, "transport_server")
	config := NewConfig(WithLogger(l))
	quicConfig := &quic.Config{
		EnableDatagrams: true,
		KeepAlivePeriod: time.Second * 3,
	}
	config.Transports = append(config.Transports,
		transport.NewTransportServerConfig(
			transport.WithName("test-transport-server-1"),
			transport.WithAddress(":9800"),
			transport.WithQuicConfig(quicConfig),
		),
		transport.NewTransportServerConfig(
			transport.WithName("test-transport-server-2"),
			transport.WithAddress(":9801"),
			transport.WithQuicConfig(quicConfig),
		),
	)

	// create and run server
	s := NewServer(config)
	err = s.Run(context.Background())
	if err != nil {
		t.Fatal(err)
		return
	}
}
