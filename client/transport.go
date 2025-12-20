package client

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
	}
	TransportType string
)

const (
	TransportTypeQuic TransportType = "quic"
)

func NewTransport(ctx context.Context, t TransportType, address string, port int, auth *auth.Handler, dev *device.Config) (Transport, error) {
	switch t {
	case TransportTypeQuic:
		return NewQuicTransport(ctx, address, port, auth, dev)
	default:
		return nil, fmt.Errorf("transport type not supported: %s", t)
	}
}
