package client

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"

	"go-vnet/common/session"
	"go-vnet/vnet/server"
)

type (
	Manager struct {
		callMu           sync.Mutex
		session          *session.Session
		sr               session.SendReceiveCloser
		sig              chan struct{}
		funcCallRespChan sync.Map
		eh               ServerEventHandler
	}
	ServerEventHandler func(ctx context.Context, event *server.ServersideEvent)
)

var (
	WithManagerCallOption = func(ctx context.Context, opts ...func(ctx context.Context) context.Context) context.Context {
		for _, opt := range opts {
			ctx = opt(ctx)
		}
		return ctx
	}
	ManagerCallNoResponse = func(ctx context.Context) context.Context {
		return context.WithValue(ctx, managerCallCtxKeyNoResponse, true)
	}
)

const (
	managerCallCtxKeyNoResponse = "no_response"
)

func NewManager(session *session.Session, sr session.SendReceiveCloser) *Manager {
	m := &Manager{
		session:          session,
		sr:               sr,
		sig:              make(chan struct{}),
		funcCallRespChan: sync.Map{},
	}
	go m.receiveLoop(context.Background())
	return m
}

func (m *Manager) Close() {
	close(m.sig)
}

func (m *Manager) receiveLoop(ctx context.Context) {
	for {
		select {
		case <-m.sig:
			return
		default:
			receive, err := m.sr.Receive(ctx)
			if err != nil {
				return
			}

			switch receive[0] {
			case server.MessageTypeFuncCall:
				resp := new(server.FuncCallResponse)
				err = json.Unmarshal(receive[1:], &resp)
				if err != nil {
					g.Log().Errorf(ctx, "failed to unmarshal func call response: %s", err.Error())
					continue
				}
				if ch, ok := m.funcCallRespChan.Load(resp.RequestId); ok {
					ch.(chan *server.FuncCallResponse) <- resp
					continue
				}
				g.Log().Infof(ctx, "func call response dropped: %s", resp.RequestId)
			case server.MessageTypeServerEvent:
				event := new(server.ServersideEvent)
				err = json.Unmarshal(receive[1:], &event)
				if err != nil {
					g.Log().Errorf(ctx, "failed to unmarshal server event: %s", err.Error())
					continue
				}
				if m.eh != nil {
					m.eh(ctx, event)
				} else {
					g.Log().Infof(ctx, "server event dropped: %s,id=%s", event.Event, event.EventId)
				}
			}
		}
	}
}

func (m *Manager) CallFunc(ctx context.Context, name string, args ...map[string]any) (resp *server.FuncCallResponse, err error) {
	m.callMu.Lock()
	defer m.callMu.Unlock()
	start := time.Now()
	defer func() {
		if resp != nil {
			resp.Cost = float64(time.Since(start).Microseconds()) / 1000
		}
	}()

	// set timeout
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Second*3)
	defer cancel()

	request := server.FuncCallRequest{
		RequestId: uuid.NewString(),
		FuncName:  name,
	}
	if len(args) > 0 {
		request.Args = args[0]
	}

	// marshal and send
	req, _ := json.Marshal(request)
	if err = m.sr.Send(req); err != nil {
		return
	}

	if ctx.Value(managerCallCtxKeyNoResponse) != nil {
		// no response
		return
	}

	// store resp channel
	respChan := make(chan *server.FuncCallResponse, 1)
	m.funcCallRespChan.Store(request.RequestId, respChan)
	defer func() {
		cancel()
		m.funcCallRespChan.Delete(request.RequestId)
	}()

	// wait for response or timeout
	select {
	case <-ctx.Done():
		err = ctx.Err()
		return
	case resp = <-respChan:
		return
	}
}

func (m *Manager) SetEventHandler(eh ServerEventHandler) {
	m.eh = eh
}
