package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	tt "go-vnet/common/tls"
	"go-vnet/device"
	"go-vnet/transport/router"
	"go-vnet/virtual_network/client"
	"go-vnet/virtual_network/server"
)

type localTestAuthorizedHandler struct {
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
					CIDR: "192.168.125.2/24",
					MTU:  1400,
				},
				Server: "127.0.0.1:8000",
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

type localTestWriter struct {
}

func (l localTestWriter) Write(p []byte) (n int, err error) {
	fmt.Printf("%v\n", p)
	return len(p), nil
}

func (l localTestWriter) Close() error {
	return nil
}

func main() {
	s := server.NewServer()
	go func() {
		if err := s.Serve(); err != nil {
			return
		}
	}()

	r := router.NewRouter()
	if err := r.Register("192.168.125.254/32", &localTestWriter{}); err != nil {
		panic(err)
		return
	}

	cfg := &server.Config{
		MappedConfig:      config.NewMappedConfig(),
		Router:            r,
		Port:              8000,
		Address:           "0.0.0.0",
		MTU:               1400,
		AuthorizedHandler: localTestAuthorizedHandler{},
	}
	tls := tt.GenerateTLSConfig(time.Hour*24*7, 1024)
	tls.InsecureSkipVerify = true
	cfg.MappedConfig.Set("tls", tls)
	cfg.MappedConfig.Set("quic_config", &quic.Config{
		KeepAlivePeriod: time.Second * 3,
		EnableDatagrams: true,
	})
	qs := server.NewQuicServer(cfg)
	err := qs.Serve(context.Background())
	if err != nil {
		panic(err)
	}
}
