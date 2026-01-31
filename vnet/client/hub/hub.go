package hub

import (
	"context"
	"runtime"

	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/logger"
	"go-vnet/common/protocol"
	"go-vnet/common/router"
)

type (
	Hub struct {
		ctx    context.Context
		cfg    Config
		sig    chan struct{}
		logger logger.Logger
		// rx
		rxEventPool *EventPool[*rxEvent]
		rxEventChan chan *rxEvent
		// tx
		txEventPool *EventPool[*txEvent]
		txEventChan chan *txEvent
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
		rxEventChan: make(chan *rxEvent, cfg.MaxRxEventBuf),
		txEventChan: make(chan *txEvent, cfg.MaxTxEventBuf),
	}

	// 初始化 rx 事件池
	hub.rxEventPool = NewEventPool[*rxEvent](128, func() *rxEvent {
		e := &rxEvent{}
		e.packet = &[protocol.MaxTransportByteSize]byte{}
		e.buf = make([][]byte, cfg.BatchSize)
		e.sizes = make([]int, cfg.BatchSize)
		for i := 0; i < cfg.BatchSize; i++ {
			e.buf[i] = make([]byte, cfg.MTU+cfg.HeaderSize)
		}
		return e
	})

	// 初始化 tx 事件池
	hub.txEventPool = NewEventPool[*txEvent](128, func() *txEvent {
		e := &txEvent{}
		e.Buffer = make([][]byte, cfg.BatchSize)
		e.Sizes = make([]int, cfg.BatchSize)
		for i := 0; i < cfg.BatchSize; i++ {
			e.Buffer[i] = make([]byte, cfg.MTU+cfg.HeaderSize)
		}
		return e
	})

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
