package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sync"

	"github.com/quic-go/quic-go"
)

type (
	SendReceiver interface {
		Send(data []byte) (err error)
		Receive(ctx context.Context) (data []byte, err error)
	}
	Manager struct {
		sig      chan struct{}
		sessions sync.Map // src : Session
	}
	quicSendReceiver struct {
		*quic.Conn
	}
)

type (
	FuncCallRequest struct {
		FuncName string         `json:"func_name"`
		Args     map[string]any `json:"args"`
	}
	FuncCallResponse struct {
		Code    int    `json:"code"`
		Data    any    `json:"data"`
		Message string `json:"message"`
	}
)

func NewManager() *Manager {
	return &Manager{
		sig: make(chan struct{}),
	}
}

// RangeReceiveLimited 并发遍历所有 Session 的 Receive 方法（限制并发数）
// maxConcurrency: 最大并发数，避免资源耗尽
func (manager *Manager) RangeReceiveLimited(ctx context.Context, maxConcurrency int, handler func(session *Session, data []byte, err error)) {
	workerPool := make(chan struct{}, maxConcurrency)
	for i := 0; i < maxConcurrency; i++ {
		workerPool <- struct{}{}
	}

	var wg sync.WaitGroup
	manager.sessions.Range(func(key, value any) bool {
		wg.Add(1)
		go func(sess *Session) {
			defer wg.Done()
			<-workerPool
			defer func() { workerPool <- struct{}{} }()

			for {
				data, err := sess.Receive(ctx)
				handler(sess, data, err)
				if ctx.Err() != nil || err != nil {
					return
				}
			}
		}(value.(*Session))
		return true
	})
	wg.Wait()
}

func (manager *Manager) ProcessFuncCallLoop(ctx context.Context) (err error) {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-manager.sig:
			return
		default:
		}

		manager.RangeReceiveLimited(ctx, 10, func(session *Session, data []byte, err error) {
			if err != nil {
				return
			}
			var req FuncCallRequest
			err = json.Unmarshal(data, &req)
			if err != nil {
				return
			}
			resp, err := manager.handleFuncCall(ctx, session, &req)
			if err != nil {
				return
			}
			respBytes, _ := json.Marshal(resp)
			_ = session.Send(respBytes)
		})
	}
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

func (manager *Manager) DeleteSession(src string) {
	manager.sessions.Delete(src)
}

func (manager *Manager) Register(src string, session *Session) {
	manager.sessions.Store(src, session)
}

func (q quicSendReceiver) Send(data []byte) (err error) {
	return q.SendDatagram(data)
}

func (q quicSendReceiver) Receive(ctx context.Context) (data []byte, err error) {
	return q.ReceiveDatagram(ctx)
}
