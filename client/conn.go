package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"go-vnet/common/logger"
	"go-vnet/server"
)

type ConnectFunc func(dst string) (io.ReadWriteCloser, error)

type ConnectionManager struct {
	ctx      context.Context
	mu       sync.Mutex
	execMu   sync.Mutex
	p        map[string]io.ReadWriteCloser // dst(ip):writer
	connFunc ConnectFunc
	logger   logger.Logger
}

func NewManager(ctx context.Context, cf ConnectFunc) *ConnectionManager {
	return &ConnectionManager{
		p:        make(map[string]io.ReadWriteCloser),
		ctx:      ctx,
		connFunc: cf,
	}
}

func (p *ConnectionManager) SetLogger(l logger.Logger) {
	p.logger = l
}

func (p *ConnectionManager) Del(dst string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if w, ok := p.p[dst]; ok {
		_ = w.Close()
	}
	delete(p.p, dst)
}

func (p *ConnectionManager) Get(dst string, noHandshake ...bool) (rwc io.ReadWriteCloser, err error) {
	defer func() {
		if err != nil {
			p.mu.Lock()
			defer p.mu.Unlock()
			p.logger.Errorf(p.ctx, "remove connection %s: %s", dst, err.Error())
			delete(p.p, dst)
		}
	}()

	if w, ok := p.p[dst]; ok {
		rwc = w
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// try read map again
	if w, ok := p.p[dst]; ok {
		rwc = w
		return
	}

	conn, err := p.connFunc(dst)
	if err != nil {
		err = fmt.Errorf("connect to server failed: %w", err)
		return
	}
	defer func() {
		if err == nil {
			p.p[dst] = conn
			rwc = conn
		}
	}()
	if len(noHandshake) > 0 && noHandshake[0] {
		return
	}

	first := []byte(dst)
	if dst == "" {
		first = []byte{0x01}
	}

	_, err = conn.Write(first)
	if err != nil {
		err = fmt.Errorf("send route pkg failed: %w", err)
		return
	}
	res := make([]byte, 1)
	read, err := conn.Read(res)
	if err != nil {
		err = fmt.Errorf("read route pkg failed: %w", err)
		return
	}
	if read != 1 || res[0] != 1 {
		if dst == "" {
			err = fmt.Errorf("no manager connection")
		} else {
			err = fmt.Errorf("no route to dst: %s", dst)
		}
		return
	}
	return
}

func (p *ConnectionManager) ExecFunc(name string, args ...map[string]any) (resp *server.FuncCallResponse, err error) {
	p.execMu.Lock()
	defer p.execMu.Unlock()

	conn, err := p.Get("")
	if err != nil {
		return
	}

	request := server.FuncCallRequest{
		FuncName: name,
	}
	if len(args) > 0 {
		request.Args = args[0]
	}

	req, _ := json.Marshal(request)

	_, err = conn.Write(req)
	if err != nil {
		return
	}

	var (
		buf   = &bytes.Buffer{}
		bs    = make([]byte, 1024)
		timer = time.NewTimer(time.Second * 10)
	)

	for {
		select {
		case <-timer.C:
			err = fmt.Errorf("exec func timeout")
			return
		default:
		}
		n, err := conn.Read(bs)
		if err != nil {
			break
		}
		buf.Write(bs[:n])
		if n < 1024 {
			break
		}
	}

	resp = new(server.FuncCallResponse)
	if err = json.Unmarshal(buf.Bytes(), &resp); err != nil {
		return
	}
	return
}

func (p *ConnectionManager) Keys() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	keys := make([]string, 0, len(p.p))
	for k := range p.p {
		keys = append(keys, k)
	}
	return keys
}
