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
		Session *Session
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

func (manager *Manager) ProcessFuncCallLoop(ctx context.Context) (err error) {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-manager.sig:
			return
		case ev := <-manager.events:
			var (
				req  FuncCallRequest
				resp *FuncCallResponse
			)

			err = json.Unmarshal(ev.Data, &req)
			if err != nil {
				return
			}
			resp, err = manager.handleFuncCall(ctx, ev.Session, &req)
			if err != nil {
				return
			}
			respBytes, _ := json.Marshal(resp)
			_ = ev.Session.Send(respBytes)
		}
	}
}

func (manager *Manager) Close() error {
	close(manager.sig)
	return nil
}

const (
	FuncNamePing          = "ping"
	FuncNameGetRouterData = "get_router_data"
)

func (manager *Manager) handleFuncCall(_ context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
	switch req.FuncName {
	case FuncNamePing:
		return &FuncCallResponse{Code: 0, Data: session.Network.Router().Hash()}, nil
	case FuncNameGetRouterData:
		data := session.Network.Router().Dump()
		return &FuncCallResponse{Code: 0, Data: base64.StdEncoding.EncodeToString(data)}, nil
	default:
	}
	return &FuncCallResponse{Code: -1, Message: "unknown func name"}, nil
}

func (manager *Manager) PushEvent(session *Session, data []byte) {
	manager.events <- &FuncCallEvent{
		Session: session,
		Data:    data,
	}
}
