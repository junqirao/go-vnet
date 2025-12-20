package client

import (
	"context"
	"fmt"
	"io"
	"sync"

	"go-vnet/common/logger"
)

type ConnectFunc func(dst string) (io.ReadWriteCloser, error)

type ConnectionManager struct {
	ctx      context.Context
	mu       sync.Mutex
	p        map[string]io.ReadWriteCloser // dst(ip):writer
	connFunc ConnectFunc
	logger   logger.Logger
}

func NewConnectionManager(ctx context.Context, cf ConnectFunc) *ConnectionManager {
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
	delete(p.p, dst)
}

func (p *ConnectionManager) Get(dst string) (rwc io.ReadWriteCloser, err error) {
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
		return
	}

	if _, err = conn.Write([]byte(dst)); err != nil {
		err = fmt.Errorf("send route pkg failed: %w", err)
		return
	}
	res := make([]byte, 1)
	read, err := conn.Read(res)
	if err != nil {
		return
	}
	if read != 1 || res[0] != 1 {
		err = fmt.Errorf("no route to dst: %s", dst)
		return
	}
	p.p[dst] = conn
	rwc = conn
	return
}

func (p *ConnectionManager) makeConnectOrWrite(dst string, data []byte) (n int, err error) {
	rwc, err := p.Get(dst)
	if err != nil {
		return
	}
	return rwc.Write(data)
}
