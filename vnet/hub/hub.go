package hub

import (
	"context"
	"fmt"
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
		// tx
		txEventPool sync.Pool
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
	return hub
}

func (h *Hub) Start() {
	go h.txLoop()
	h.rxLoop()
}

func (h *Hub) Stop(reason ...string) {
	close(h.sig)
	if len(reason) > 0 {
		fmt.Println(reason[0])
	}
	fmt.Println("hub stopped")
}
