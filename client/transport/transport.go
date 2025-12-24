package transport

import (
	"context"
	"fmt"
	"io"

	"go-vnet/common/auth"
	"go-vnet/device"
)

type (
	Transport interface {
		Connect(dst string) (io.ReadWriteCloser, error)
		JoinedNetwork() JoinNetworkResponse
	}
	JoinNetworkResponse struct {
		Device device.Config `json:"device"`
	}
)

func NewTransport(ctx context.Context, cfg *Config, auth *auth.Client) (Transport, error) {
	switch cfg.Type {
	case TypeQuic:
		return NewQuicTransport(ctx, cfg, auth)
	default:
		return nil, fmt.Errorf("transport type not supported: %s", cfg.Type)
	}
}
