package auth

import (
	"context"
)

type (
	Encoder interface {
		Encode(ctx context.Context, payload map[string]any) (data []byte, err error)
		Decode(ctx context.Context, in []byte) (payload map[string]any, err error)
		DecodeTo(ctx context.Context, in []byte, ptr any) (err error)
	}
)
