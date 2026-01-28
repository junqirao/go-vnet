package server

import (
	"context"
	"encoding/json"
	"sync"
)

type (
	Manager struct {
		sig       chan struct{}
		functions sync.Map // name :  FuncCallHandler
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
	FuncCallHandler func(ctx context.Context, session *serverSession, req *FuncCallRequest) (resp *FuncCallResponse, err error)
	FuncCallInfo    struct {
		Name string
		Fn   FuncCallHandler
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
	FuncNamePing            = "ping"
	FuncNameGetRouterData   = "get_router_data"
	FuncNameGetP2PRelayInfo = "get_p2p_relay_info"
)

func (manager *Manager) handleFuncCall(ctx context.Context, session *serverSession, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
	value, ok := manager.functions.Load(req.FuncName)
	if ok {
		if f, ok := value.(FuncCallHandler); ok {
			return f(ctx, session, req)
		}
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

func (manager *Manager) RegisterHandler(info ...FuncCallInfo) {
	for _, callInfo := range info {
		manager.functions.Store(callInfo.Name, callInfo.Fn)
	}
}
