package server

import (
	"context"
	"io"

	"go-vnet/common/router"
)

type Transport struct {
	ctx context.Context
	r   router.Router
	dst map[string]io.WriteCloser
}
