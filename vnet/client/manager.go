package client

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go-vnet/vnet/server"
	"go-vnet/vnet/session"
)

type (
	Manager struct {
		callMu  sync.Mutex
		session *session.Session
		sr      session.SendReceiveCloser
	}
)

func NewManager(session *session.Session, sr session.SendReceiveCloser) *Manager {
	return &Manager{
		session: session,
		sr:      sr,
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

	if err = manager.sr.Send(req); err != nil {
		return
	}

	receive, err := manager.sr.Receive(ctx)
	if err != nil {
		return
	}

	resp = new(server.FuncCallResponse)
	err = json.Unmarshal(receive, &resp)
	return
}
