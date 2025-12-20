package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"go-vnet/client"
	"go-vnet/common/auth"
	"go-vnet/common/logger"
	"go-vnet/common/router"
	"go-vnet/device"
)

type localTestAuthorizedHandler struct {
	cidr   string
	server string
}

func (t localTestAuthorizedHandler) Make(ctx context.Context, payload ...map[string]any) (data []byte, err error) {
	if len(payload) > 0 && payload[0] != nil {
		data, err = json.Marshal(payload[0])
	} else {
		data = make([]byte, 0)
	}
	return
}

func (t localTestAuthorizedHandler) Handle(ctx context.Context, in []byte) (payload map[string]any, err error) {
	logger.DefaultLogger.Infof(ctx, "receive payload: %v", string(in))
	payload = map[string]any{}
	_ = json.Unmarshal(in, &payload)
	if fn, ok := payload["func_name"]; ok && fn != "" {
		switch fn {
		case "join_network":
			payload["data"] = client.JoinNetworkResponse{
				Device: device.Config{
					Name: "test_device",
					CIDR: t.cidr,
					MTU:  1400,
				},
				Server: t.server,
			}
		}
	}

	return
}

func (t localTestAuthorizedHandler) Do(ctx context.Context, au auth.AuthorizedHandler, send ...map[string]any) (receive map[string]any, err error) {
	authData, err := au.Make(ctx, send...)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	receive, err = au.Handle(ctx, authData)
	return
}

func main() {
	cidr := flag.String("cidr", "", "192.168.125.2/24")
	server := flag.String("server", "", "172.18.28.101:8000")
	flag.Parse()

	lta := localTestAuthorizedHandler{
		cidr:   *cidr,
		server: *server,
	}
	c, err := client.NewClientWithAuthorizedHandler("test_network_id", auth.NewHandler(lta, lta))
	if err != nil {
		panic(err)
		return
	}

	rt := router.NewRouter()
	err = rt.Register("192.168.125.254/32", "ok")
	if err != nil {
		panic(err)
		return
	}
	c.SetLogger(logger.DefaultLogger)
	c.SetRouter(rt)

	err = c.Run(context.Background())
	if err != nil {
		panic(err)
		return
	}
}
