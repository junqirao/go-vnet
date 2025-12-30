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
		sessions sync.Map   // src : Session
		notEmpty *sync.Cond // 用于在 map 为空时等待
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
		Cost    int64  `json:"cost"`
	}
)

func NewManager() *Manager {
	m := &Manager{
		sig: make(chan struct{}),
	}
	m.notEmpty = sync.NewCond(&sync.Mutex{})
	return m
}

// RangeReceiveLimited 并发遍历所有 Session 的 Receive 方法（限制并发数）
// maxConcurrency: 最大并发数，避免资源耗尽
// 每次只接收一个消息，避免阻塞 ProcessFuncCallLoop
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

			// 只接收一次消息，不阻塞
			data, err := sess.Receive(ctx)
			handler(sess, data, err)
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

		// 如果 sessions 为空，等待直到有新 session 注册
		if !manager.hasSession() {
			manager.waitForSessionOrCtx(ctx)
			continue
		}

		// 处理当前所有 session 的请求
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
	manager.notEmpty.Broadcast() // 通知等待的 goroutine 有新 session
}

func (q quicSendReceiver) Send(data []byte) (err error) {
	return q.SendDatagram(data)
}

func (q quicSendReceiver) Receive(ctx context.Context) (data []byte, err error) {
	return q.ReceiveDatagram(ctx)
}

func (manager *Manager) hasSession() bool {
	has := false
	manager.sessions.Range(func(key, value any) bool {
		has = true
		return false // 只需要找到一个就停止
	})
	return has
}

func (manager *Manager) waitForSessionOrCtx(ctx context.Context) {
	manager.notEmpty.L.Lock()
	defer manager.notEmpty.L.Unlock()

	// 在等待前再次检查，避免 race condition
	if manager.hasSession() {
		return
	}

	// 使用 goroutine 来支持 context 取消
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.notEmpty.Wait()
	}()

	select {
	case <-done:
		// 被 Broadcast 唤醒
	case <-ctx.Done():
		// context 被取消，手动唤醒等待的 goroutine
		manager.notEmpty.Broadcast()
	}
}
