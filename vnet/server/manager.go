package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"
)

type (
	Manager struct {
		sig        chan struct{}
		pingRecord sync.Map // src -> time.Time
	}
)

type (
	FuncCallRequest struct {
		FuncName string         `json:"func_name"`
		Args     map[string]any `json:"args"`
	}
	FuncCallResponse struct {
		Code    int     `json:"code"`
		Data    any     `json:"data"`
		Message string  `json:"message"`
		Cost    float64 `json:"cost"`
	}
)

func NewManager() *Manager {
	m := &Manager{
		sig: make(chan struct{}),
	}
	return m
}

func (manager *Manager) Close() error {
	close(manager.sig)
	return nil
}

const (
	FuncNamePing          = "ping"
	FuncNameGetRouterData = "get_router_data"
)

func (manager *Manager) handleFuncCall(_ context.Context, session *serverSession, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
	switch req.FuncName {
	case FuncNamePing:
		manager.pingRecord.Store(session.IP, time.Now())
		return &FuncCallResponse{Code: 0, Data: session.network.Router().MD5()}, nil
	case FuncNameGetRouterData:
		data := session.network.Router().Keys()
		bs, _ := json.Marshal(data)
		return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(bs)}, nil
	default:
	}
	return &FuncCallResponse{Code: -1, Message: "unknown func name"}, nil
}

func (manager *Manager) HandleEvent(ctx context.Context, session *serverSession, data []byte) (err error) {
	var (
		req  FuncCallRequest
		resp *FuncCallResponse
	)

	err = json.Unmarshal(data, &req)
	if err != nil {
		return
	}
	resp, err = manager.handleFuncCall(ctx, session, &req)
	if err != nil {
		return
	}
	respBytes, _ := json.Marshal(resp)
	return session.Send(respBytes)
}
