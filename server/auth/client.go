package auth

import (
	"context"
)

type Client struct {
	encoder Encoder
}

type (
	ClientHandlerFunc func(ctx context.Context, in []byte) (out []byte, err error)
)

func NewClient(encoder Encoder) *Client {
	return &Client{encoder: encoder}
}

func (c *Client) Auth(ctx context.Context, request map[string]any, handler ClientHandlerFunc) (resp map[string]any, err error) {
	in, err := c.encoder.Encode(ctx, request)
	if err != nil {
		return
	}
	out, err := handler(ctx, in)
	if err != nil {
		return
	}

	return c.encoder.Decode(ctx, out)
}
