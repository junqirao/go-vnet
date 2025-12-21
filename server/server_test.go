package server

import (
	"context"
	"testing"

	"go-vnet/common/logger"
	"go-vnet/server/transport"
)

func TestServer_Run(t *testing.T) {
	l := logger.NewJsonLogger(nil, "transport_server")
	config := NewConfig(WithLogger(l))
	config.Transports = append(config.Transports,
		transport.NewTransportServerConfig(
			transport.WithName("test-transport-server-1"),
			transport.WithAddress(":9800"),
		),
		transport.NewTransportServerConfig(
			transport.WithName("test-transport-server-2"),
			transport.WithAddress(":9801"),
		),
	)
	s := NewServer(config)
	err := s.Run(context.Background())
	if err != nil {
		t.Fatal(err)
		return
	}
}
