package auth

import (
	"context"
	"encoding/json"
	"fmt"
)

type (
	HandleFunc interface {
		Do(ctx context.Context, au AuthorizedHandler, send ...map[string]any) (receive map[string]any, err error)
	}
	Handler struct {
		au      AuthorizedHandler
		handler HandleFunc
	}
)

func NewHandler(au AuthorizedHandler, handler HandleFunc) *Handler {
	return &Handler{au: au, handler: handler}
}

func (i *Handler) Do(ctx context.Context, send ...map[string]any) (receive map[string]any, err error) {
	return i.handler.Do(ctx, i.au, send...)
}

func (i *Handler) DoStructPtr(ctx context.Context, receivePtr any, send ...map[string]any) (err error) {
	res, err := i.handler.Do(ctx, i.au, send...)
	if err != nil {
		return
	}

	bs, err := json.Marshal(res)
	if err != nil {
		return
	}

	err = json.Unmarshal(bs, receivePtr)
	return
}

func (i *Handler) Call(ctx context.Context, name string, send map[string]any) (receive any, err error) {
	res, err := i.handler.Do(ctx, i.au, map[string]any{
		"func_name": name,
		"payload":   send,
	})
	if err != nil {
		return
	}
	if code, ok := res["code"]; ok && code != 0 {
		err = fmt.Errorf("execute %s failed: code=%v message=%v", name, code, res["message"])
		return
	}
	receive = res["data"]
	return
}

func (i *Handler) CallPtr(ctx context.Context, name string, send map[string]any, receivePtr any) (err error) {
	res, err := i.Call(ctx, name, send)
	if err != nil {
		return
	}

	bs, err := json.Marshal(res)
	if err != nil {
		return
	}

	err = json.Unmarshal(bs, receivePtr)
	return
}

func (i *Handler) AuthorizedHandler() AuthorizedHandler {
	return i.au
}
