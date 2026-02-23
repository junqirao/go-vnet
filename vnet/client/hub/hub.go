package hub

import (
	"context"
	"errors"
	"runtime"

	"github.com/gogf/gf/v2/frame/g"
	tun "github.com/sagernet/sing-tun"

	"go-vnet/common/protocol"
	"go-vnet/common/router"
)

var (
	P2PNotEnabled = errors.New("p2p not enabled")
)

const (
	DeviceModeProxyOnly DeviceMode = 1 << iota
	DeviceModeP2POnly
	DeviceModeMixed
)

func DeviceModeFromString(s string) DeviceMode {
	switch s {
	case "proxy_only":
		return DeviceModeProxyOnly
	case "p2p_only":
		return DeviceModeP2POnly
	case "mixed":
		return DeviceModeMixed
	default:
		return DeviceModeMixed
	}
}

func (m DeviceMode) String() string {
	switch m {
	case DeviceModeProxyOnly:
		return "proxy_only"
	case DeviceModeP2POnly:
		return "p2p_only"
	case DeviceModeMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

type (
	DeviceMode uint8
	Hub        struct {
		ctx context.Context
		cfg Config
		sig chan struct{}
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
		// latency
		latencyManager *LatencyManager
	}
	Config struct {
		Name          string     `json:"-"`
		BatchSize     int        `json:"-"`
		HeaderSize    int        `json:"-"`
		MTU           int        `json:"-"`
		MaxRxEventBuf int        `json:"max_rx_event_buf"`
		MaxTxEventBuf int        `json:"max_tx_event_buf"`
		Mode          DeviceMode `json:"mode"`
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

	// 初始化延迟监控管理器
	latencyManager, err := NewLatencyManager(ctx, hub, DefaultLatencyManagerConfig())
	if err != nil {
		g.Log().Warningf(ctx, "create latency manager failed: %v", err)
		latencyManager = nil
	} else {
		hub.latencyManager = latencyManager
	}

	return hub
}

func (h *Hub) Start() {
	// 启动延迟监控管理器
	if h.latencyManager != nil {
		h.latencyManager.Start(DefaultLatencyManagerConfig())
	}

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

	// 停止延迟监控管理器
	if h.latencyManager != nil {
		h.latencyManager.Stop()
	}

	close(h.sig)
	if len(reason) > 0 {
		g.Log().Errorf(h.ctx, "hub stopped reason: %s", reason[0])
	} else {
		g.Log().Infof(h.ctx, "hub stopped")
	}
}

func (h *Hub) Router() *router.Router {
	return h.router
}

func (h *Hub) LatencyManager() *LatencyManager {
	return h.latencyManager
}
