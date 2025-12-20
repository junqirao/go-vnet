package auth

import (
	"context"
)

type AuthorizedHandler interface {
	Make(ctx context.Context, payload ...map[string]any) (data []byte, err error)
	Handle(ctx context.Context, in []byte) (payload map[string]any, err error)
}

type NopAuthorizedHandler struct{}

func (NopAuthorizedHandler) Make(ctx context.Context, payload ...map[string]any) (data []byte, err error) {
	return
}

func (NopAuthorizedHandler) Handle(ctx context.Context, in []byte) (meta map[string]any, err error) {
	return
}
