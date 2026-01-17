package auth

import (
	"context"
)

type Client struct {
	Encoder
}

type (
	ClientHandlerFunc func(ctx context.Context, in []byte) (out []byte, err error)
)

func NewClient(cfg Config, encoder ...Encoder) *Client {
	c := &Client{}
	if len(encoder) > 0 {
		c.Encoder = encoder[0]
		return c
	}
	switch cfg.Type {
	case TypeRSA:
		var opts []RSAEncoderOption
		if cfg.PublicKey != "" {
			opts = append(opts, WithPublicKey(cfg.PublicKey))
		}
		if cfg.PrivateKey != "" {
			opts = append(opts, WithPrivateKey(cfg.PrivateKey))
		}
		c.Encoder = NewRsaEncoder(opts...)
	default:
		c.Encoder = NewSimplePasswordEncoder(cfg.Password)
	}
	return c
}

func (c *Client) Auth(ctx context.Context, request map[string]any, handler ClientHandlerFunc) (resp map[string]any, err error) {
	in, err := c.Encode(ctx, request)
	if err != nil {
		return
	}
	out, err := handler(ctx, in)
	if err != nil {
		return
	}

	return c.Decode(ctx, out)
}

func (c *Client) AuthPtr(ctx context.Context, request map[string]any, handler ClientHandlerFunc, ptr any) (err error) {
	in, err := c.Encode(ctx, request)
	if err != nil {
		return
	}
	out, err := handler(ctx, in)
	if err != nil {
		return
	}

	return c.DecodeTo(ctx, out, &ptr)
}
