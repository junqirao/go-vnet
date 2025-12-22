package transport

import (
	"context"
	"fmt"
	"io"

	"go-vnet/device"
	"go-vnet/server/auth"
)

type (
	Transport interface {
		Connect(dev *device.Config, dst string) (io.ReadWriteCloser, error)
	}
	Type string
)

const (
	TypeQuic Type = "quic"
)

func NewTransport(ctx context.Context, t Type, address string, port int, auth *auth.Client) (Transport, error) {
	switch t {
	case TypeQuic:
		return NewQuicTransport(ctx, address, port, auth)
	default:
		return nil, fmt.Errorf("transport type not supported: %s", t)
	}
}
