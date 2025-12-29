package server

import (
	"context"
	"sync"
)

type (
	SendReceiver interface {
		Send(data []byte) (err error)
		Receive(ctx context.Context) (data []byte, err error)
	}
	Manager struct {
		sessions sync.Map // src : Session
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
