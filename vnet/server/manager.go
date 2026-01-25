package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
)

type (
	Manager struct {
		sig    chan struct{}
		events chan *FuncCallEvent
	}
	FuncCallEvent struct {
		Session *serverSession
		Data    []byte
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
		sig:    make(chan struct{}),
		events: make(chan *FuncCallEvent, 1024),
	}
	return m
}

// func (manager *Manager) ProcessFuncCallLoop(ctx context.Context) (err error) {
// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		case <-manager.sig:
// 			return
// 		case ev := <-manager.events:
// 			_ = manager.HandleEvent(ctx, ev.Session, ev.Data)
// 		}
// 	}
// }

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
		return &FuncCallResponse{Code: 0, Data: session.network.Router().MD5()}, nil
	case FuncNameGetRouterData:
		data := session.network.Router().Keys()
		bs, _ := json.Marshal(data)
		return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(bs)}, nil
	default:
	}
	return &FuncCallResponse{Code: -1, Message: "unknown func name"}, nil
}

// func (manager *Manager) HandleEventAsync(session *serverSession, data []byte) {
// 	manager.events <- &FuncCallEvent{
// 		Session: session,
// 		Data:    data,
// 	}
// }

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
