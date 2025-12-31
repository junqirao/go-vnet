package client

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go-vnet/server"
)

type (
	SendReceiver interface {
		Send(data []byte) (err error)
		Receive(ctx context.Context) (data []byte, err error)
	}
	Manager struct {
		callMu  sync.Mutex
		session *Session
	}
)

func NewManager(session *Session) *Manager {
	return &Manager{
		session: session,
	}
}

func (manager *Manager) CallFunc(ctx context.Context, name string, args ...map[string]any) (resp *server.FuncCallResponse, err error) {
	manager.callMu.Lock()
	defer manager.callMu.Unlock()
	start := time.Now()
	defer func() {
		if resp != nil {
			resp.Cost = float64(time.Since(start).Microseconds()) / 1000
		}
	}()

	request := server.FuncCallRequest{
		FuncName: name,
	}
	if len(args) > 0 {
		request.Args = args[0]
	}

	req, _ := json.Marshal(request)

	if err = manager.session.Send(req); err != nil {
		return
	}

	receive, err := manager.session.Receive(ctx)
	if err != nil {
		return
	}

	resp = new(server.FuncCallResponse)
	err = json.Unmarshal(receive, &resp)
	return
}
