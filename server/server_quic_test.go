package server

import (
	"context"
	"fmt"
	"testing"

	"go-vnet/common/auth"
	"go-vnet/common/logger"
	"go-vnet/server/network"
)

func TestServe(t *testing.T) {
	// create network
	n, err := network.NewNetwork(&network.Config{
		ID:         "test",
		CIDR:       "192.168.98.0/24",
		RouterData: nil,
		MTU:        1400,
	})
	if err != nil {
		t.Fatal(err)
		return
	}

	// register network
	network.GetManager().RegisterNetwork(n)

	l := logger.NewStdLogger(nil, "transport_server")
	cfg := NewConfig(
		WithLogger(l),
		WithAddress("0.0.0.0:9800"),
		WithAuthConfig(auth.Config{
			Type:     auth.TypeSimplePassword,
			Password: "",
		}),
		WithAuthenticationChainFunc(func(ctx context.Context, request map[string]any, resp map[string]any) (err error) {
			fmt.Println("------------")
			fmt.Println("Auth Chain Func")
			fmt.Printf("%+v\n", request)
			fmt.Println("------------")
			return nil
		}),
	)
	server := newQuicServer(cfg)
	err = server.Serve(context.Background())
	if err != nil {
		t.Fatal(err)
		return
	}
}
