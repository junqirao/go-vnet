package hub

import (
	"context"
	"runtime"
	"sync"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/logger"
	"go-vnet/vnet/protocol"
	"go-vnet/vnet/router"
)

type (
	Hub struct {
		ctx    context.Context
		cfg    Config
		sig    chan struct{}
		logger logger.Logger
		// rx
		rxEventPool sync.Pool
		rxEventChan chan *rxEvent
		rxEventCh   chan *rxEvent // 预填充缓冲区（channel 方案，并发安全）
		// tx
		txEventPool sync.Pool
		txEventChan chan *txEvent
		txEventCh   chan *txEvent // 预填充缓冲区（channel 方案，并发安全）
		// tun
		dev tun.Tun
		// router
		router *router.Router
	}
	Config struct {
		Name          string `json:"-"`
		BatchSize     int    `json:"-"`
		HeaderSize    int    `json:"-"`
		MTU           int    `json:"-"`
		MaxRxEventBuf int    `json:"max_rx_event_buf"`
		MaxTxEventBuf int    `json:"max_tx_event_buf"`
	}
)

func NewHub(cfg Config, dev tun.Tun) *Hub {
	switch runtime.GOOS {
	case "windows":
		cfg.BatchSize = 128
	case "linux":
		lt := dev.(tun.LinuxTUN)
		cfg.BatchSize = lt.BatchSize()
		cfg.HeaderSize = lt.FrontHeadroom()
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, "hub", cfg.Name)
	hub := &Hub{
		ctx:         ctx,
		cfg:         cfg,
		dev:         dev,
		sig:         make(chan struct{}),
		logger:      logger.DefaultLogger,
		router:      router.NewRouter(),
		rxEventPool: sync.Pool{},
		rxEventChan: make(chan *rxEvent, cfg.MaxRxEventBuf),
		txEventPool: sync.Pool{},
		txEventChan: make(chan *txEvent, cfg.MaxTxEventBuf),
	}
	hub.rxEventPool.New = func() any {
		e := &rxEvent{}
		e.packet = &[protocol.MaxTransportByteSize]byte{}
		e.buf = make([][]byte, cfg.BatchSize)
		e.sizes = make([]int, cfg.BatchSize)
		for i := 0; i < cfg.BatchSize; i++ {
			e.buf[i] = make([]byte, cfg.MTU+cfg.HeaderSize)
		}
		return e
	}
	hub.txEventPool.New = func() any {
		e := &txEvent{}
		e.Buffer = make([][]byte, cfg.BatchSize)
		e.Sizes = make([]int, cfg.BatchSize)
		for i := 0; i < cfg.BatchSize; i++ {
			e.Buffer[i] = make([]byte, cfg.MTU+cfg.HeaderSize)
		}
		return e
	}

	// 预填充 channel 缓冲区，保证池内一直保持一定数量的对象
	// channel 天然并发安全，避免 slice 的竞态条件和扩容问题
	minPoolSize := 256 // 可根据负载调整
	hub.rxEventCh = make(chan *rxEvent, minPoolSize)
	hub.txEventCh = make(chan *txEvent, minPoolSize)
	for i := 0; i < minPoolSize; i++ {
		hub.rxEventCh <- hub.rxEventPool.Get().(*rxEvent)
		hub.txEventCh <- hub.txEventPool.Get().(*txEvent)
	}

	return hub
}

func (h *Hub) Start() {
	go h.txLoop()
	h.rxLoop()
}

func (h *Hub) Stop(reason ...string) {
	select {
	case _, ok := <-h.sig:
		if !ok {
			return
		}
	default:
	}
	close(h.sig)
	if len(reason) > 0 {
		h.logger.Errorf(h.ctx, "hub stopped reason: %s", reason[0])
	} else {
		h.logger.Infof(h.ctx, "hub stopped")
	}
}

func (h *Hub) Router() *router.Router {
	return h.router
}
