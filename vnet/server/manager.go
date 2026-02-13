package server

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
)

const (
	MessageTypeFuncCall    byte = 0x1
	MessageTypeServerEvent byte = 0x2
)

type (
	Manager struct {
		sig       chan struct{}
		functions sync.Map // name :  FuncCallHandler
	}
)

type (
	FuncCallRequest struct {
		RequestId string         `json:"request_id"`
		FuncName  string         `json:"func_name"`
		Args      map[string]any `json:"args"`
	}
	FuncCallResponse struct {
		RequestId string  `json:"request_id"`
		Code      int     `json:"code"`
		Data      any     `json:"data"`
		Message   string  `json:"message"`
		Cost      float64 `json:"cost"`
	}
	FuncCallHandler func(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error)
	FuncCallInfo    struct {
		Name string
		Fn   FuncCallHandler
	}
)

type (
	ServersideEvent struct {
		EventId string `json:"event_id"`
		Event   string `json:"event"`
		Data    any    `json:"data"`
	}
)

func NewManager() *Manager {
	m := &Manager{
		sig: make(chan struct{}),
	}
	return m
}

func (m *Manager) Close() error {
	close(m.sig)
	return nil
}

func (m *Manager) handleFuncCall(ctx context.Context, session *Session, req *FuncCallRequest) (resp *FuncCallResponse, err error) {
	value, ok := m.functions.Load(req.FuncName)
	if ok {
		if f, ok := value.(FuncCallHandler); ok {
			resp, err = f(ctx, session, req)
			if resp != nil {
				resp.RequestId = req.RequestId
			}
			return
		}
	}
	return &FuncCallResponse{RequestId: req.RequestId, Code: -1, Message: "unknown func name"}, nil
}

func (m *Manager) HandleFuncCallEvent(ctx context.Context, session *Session, data []byte) (err error) {
	var (
		req  FuncCallRequest
		resp *FuncCallResponse
	)

	err = json.Unmarshal(data, &req)
	if err != nil {
		return
	}
	resp, err = m.handleFuncCall(ctx, session, &req)
	if err != nil {
		return
	}
	bs, _ := json.Marshal(resp)
	return session.Send(append([]byte{MessageTypeFuncCall}, bs...))
}

func (m *Manager) RegisterHandler(info ...FuncCallInfo) {
	for _, callInfo := range info {
		m.functions.Store(callInfo.Name, callInfo.Fn)
	}
}

func (m *Manager) PushEvent(session *Session, event string, data any) error {
	bs, _ := json.Marshal(ServersideEvent{
		EventId: uuid.NewString(),
		Event:   event,
		Data:    data,
	})
	return session.Send(append([]byte{MessageTypeServerEvent}, bs...))
}
