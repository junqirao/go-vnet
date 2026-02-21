package hub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// LatencyManager 延迟监控管理器
// 使用单一协程管理所有Destination的ping，避免启动过多goroutine
type LatencyManager struct {
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	hub     *Hub
	slots   map[string]*LatencySlot // key: destination IP
	stopped bool
	ticker  *time.Ticker
	wg      sync.WaitGroup
}

// LatencySlot 延迟监控槽
// 记录单个Destination的延迟监控状态和统计数据
type LatencySlot struct {
	ip             string
	pingAddr       string
	client         *PingClient
	samples        []time.Duration
	index          int
	count          int
	minLatency     time.Duration
	maxLatency     time.Duration
	avgLatency     time.Duration
	currentLatency time.Duration
	totalLatency   time.Duration
	config         *LatencyMonitorConfig
	lastPingTime   time.Time
	lastSuccess    bool
	errorCount     int
}

// LatencyMonitorConfig 延迟监控配置
// 单个Destination的ping配置
type LatencyMonitorConfig struct {
	// CheckInterval 检查间隔
	CheckInterval time.Duration
	// Timeout 超时时间
	Timeout time.Duration
	// WindowSize 统计窗口大小
	WindowSize int
}

// DefaultLatencyMonitorConfig 默认监控配置
func DefaultLatencyMonitorConfig() *LatencyMonitorConfig {
	return &LatencyMonitorConfig{
		CheckInterval: 5 * time.Second,
		Timeout:       3 * time.Second,
		WindowSize:    60,
	}
}

// LatencyManagerConfig 延迟管理器配置
type LatencyManagerConfig struct {
	// CheckInterval 检查间隔
	CheckInterval time.Duration
	// Timeout 超时时间
	Timeout time.Duration
	// WindowSize 统计窗口大小
	WindowSize int
}

// DefaultLatencyManagerConfig 默认管理器配置
func DefaultLatencyManagerConfig() *LatencyManagerConfig {
	return &LatencyManagerConfig{
		CheckInterval: 10 * time.Second,
		Timeout:       1 * time.Second,
		WindowSize:    60,
	}
}

// NewLatencyManager 创建延迟监控管理器
func NewLatencyManager(ctx context.Context, hub *Hub, cfg *LatencyManagerConfig) (*LatencyManager, error) {
	if hub == nil {
		return nil, fmt.Errorf("hub is nil")
	}
	if cfg == nil {
		cfg = DefaultLatencyManagerConfig()
	}

	managerCtx, cancel := context.WithCancel(ctx)

	m := &LatencyManager{
		ctx:     managerCtx,
		cancel:  cancel,
		hub:     hub,
		slots:   make(map[string]*LatencySlot),
		stopped: true,
	}

	return m, nil
}

// Start 启动延迟监控管理器
func (m *LatencyManager) Start(cfg *LatencyManagerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.stopped {
		return
	}

	if cfg != nil {
		m.ticker = time.NewTicker(cfg.CheckInterval)
	} else {
		cfg = DefaultLatencyManagerConfig()
		m.ticker = time.NewTicker(cfg.CheckInterval)
	}

	m.stopped = false

	m.wg.Add(1)
	go m.monitorLoop(cfg)

	g.Log().Infof(m.ctx, "[LATENCY-MANAGER] started: interval=%v", cfg.CheckInterval)
}

// Stop 停止延迟监控管理器
func (m *LatencyManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return
	}
	m.stopped = true

	m.cancel()
	if m.ticker != nil {
		m.ticker.Stop()
	}

	// 关闭所有ping客户端
	for _, slot := range m.slots {
		if slot.client != nil {
			slot.client.Close()
		}
	}
	m.slots = make(map[string]*LatencySlot)

	m.wg.Wait()

	g.Log().Infof(m.ctx, "[LATENCY-MANAGER] stopped")
}

// Register 注册需要监控的Destination
func (m *LatencyManager) Register(ip string, pingAddr string, cfg *LatencyMonitorConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果已经存在，直接返回
	if _, exists := m.slots[ip]; exists {
		return nil
	}

	// 创建ping客户端
	client, err := NewPingClient(m.ctx, pingAddr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("create ping client failed: %w", err)
	}

	// 创建监控槽
	if cfg == nil {
		cfg = DefaultLatencyMonitorConfig()
	}

	slot := &LatencySlot{
		ip:           ip,
		pingAddr:     pingAddr,
		client:       client,
		samples:      make([]time.Duration, cfg.WindowSize),
		config:       cfg,
		lastPingTime: time.Time{},
		lastSuccess:  false,
		errorCount:   0,
	}

	m.slots[ip] = slot

	g.Log().Infof(m.ctx, "[LATENCY-MANAGER] registered: ip=%s, pingAddr=%s", ip, pingAddr)
	return nil
}

// Unregister 注销Destination
func (m *LatencyManager) Unregister(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	slot, exists := m.slots[ip]
	if !exists {
		return
	}

	// 关闭ping客户端
	if slot.client != nil {
		slot.client.Close()
	}

	delete(m.slots, ip)

	g.Log().Infof(m.ctx, "[LATENCY-MANAGER] unregistered: ip=%s", ip)
}

// GetStats 获取指定Destination的延迟统计
func (m *LatencyManager) GetStats(ip string) LatencyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slot, exists := m.slots[ip]
	if !exists {
		return LatencyStats{}
	}

	return LatencyStats{
		Current:     slot.currentLatency,
		Min:         slot.minLatency,
		Max:         slot.maxLatency,
		Avg:         slot.avgLatency,
		SampleCount: slot.count,
	}
}

// GetPercentile 获取指定Destination的百分位数延迟
func (m *LatencyManager) GetPercentile(ip string, p float64) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slot, exists := m.slots[ip]
	if !exists {
		return 0
	}

	if slot.count == 0 {
		return 0
	}

	// 复制并排序样本
	sorted := make([]time.Duration, slot.count)
	for i := 0; i < slot.count; i++ {
		sorted[i] = slot.samples[i]
	}

	// 简单排序
	for i := 0; i < slot.count; i++ {
		for j := i + 1; j < slot.count; j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// 计算百分位数
	index := int(float64(slot.count) * p / 100)
	if index >= slot.count {
		index = slot.count - 1
	}

	return sorted[index]
}

// monitorLoop 监控循环
// 单一协程轮询所有注册的Destination
func (m *LatencyManager) monitorLoop(cfg *LatencyManagerConfig) {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return

		case <-m.ticker.C:
			m.pingAllDestinations(cfg)
		}
	}
}

// pingAllDestinations 批量ping所有Destination
func (m *LatencyManager) pingAllDestinations(cfg *LatencyManagerConfig) {
	m.mu.RLock()
	// 检查是否已停止
	stopped := m.stopped
	// 复制slots列表，避免在锁中执行ping
	slotsCopy := make([]*LatencySlot, 0, len(m.slots))
	for _, slot := range m.slots {
		slotsCopy = append(slotsCopy, slot)
	}
	m.mu.RUnlock()

	// 如果已停止，不执行ping
	if stopped {
		return
	}

	// 在锁外执行ping
	for _, slot := range slotsCopy {
		m.pingDestination(slot, cfg)
	}
}

// pingDestination ping单个Destination
func (m *LatencyManager) pingDestination(slot *LatencySlot, _ *LatencyManagerConfig) {
	// 检查是否已停止
	m.mu.RLock()
	stopped := m.stopped
	m.mu.RUnlock()
	if stopped {
		return
	}

	// 执行ping
	startTime := time.Now()
	rtt, err := slot.client.Ping(m.ctx, nil)

	// 检查是否已停止（可能在ping过程中停止）
	m.mu.RLock()
	stopped = m.stopped
	m.mu.RUnlock()
	if stopped {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	slot.lastPingTime = startTime

	// 确定要记录的延迟值：超时或失败时记录为0
	recordRTT := rtt
	if err != nil {
		slot.errorCount++
		slot.lastSuccess = false
		recordRTT = 0 // 超时或失败记录为0
		slot.currentLatency = 0
		g.Log().Debugf(m.ctx, "[LATENCY-MANAGER] ping failed: ip=%s, err=%v, errorCount=%d",
			slot.ip, err, slot.errorCount)
	} else {
		// 成功ping，重置错误计数
		slot.errorCount = 0
		slot.lastSuccess = true
		slot.currentLatency = rtt
	}

	// 获取被覆盖的旧样本值（用于修正totalLatency）
	oldValue := slot.samples[slot.index]
	windowFull := slot.count >= len(slot.samples)

	// 记录样本
	slot.samples[slot.index] = recordRTT
	slot.index = (slot.index + 1) % len(slot.samples)
	if slot.count < len(slot.samples) {
		slot.count++
	}

	// 更新统计（正确处理滑动窗口）
	if windowFull {
		// 窗口已满，需要减去被覆盖的旧值
		slot.totalLatency = slot.totalLatency - oldValue + recordRTT
	} else {
		// 窗口未满，直接累加
		slot.totalLatency += recordRTT
	}
	slot.avgLatency = slot.totalLatency / time.Duration(slot.count)

	// 更新min/max（只统计成功的ping，忽略0值超时记录）
	if recordRTT > 0 {
		if slot.count == 1 {
			slot.minLatency = recordRTT
			slot.maxLatency = recordRTT
		} else {
			if recordRTT < slot.minLatency || slot.minLatency == 0 {
				slot.minLatency = recordRTT
			}
			if recordRTT > slot.maxLatency {
				slot.maxLatency = recordRTT
			}
		}
	}

	// g.Log().Debugf(m.ctx, "[LATENCY-MANAGER] record: ip=%s, rtt=%v, min=%v, max=%v, avg=%v",
	// 	slot.ip, rtt, slot.minLatency, slot.maxLatency, slot.avgLatency)
}

// GetAllStats 获取所有Destination的延迟统计
func (m *LatencyManager) GetAllStats() map[string]LatencyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]LatencyStats, len(m.slots))
	for ip, slot := range m.slots {
		stats[ip] = LatencyStats{
			Current:     slot.currentLatency,
			Min:         slot.minLatency,
			Max:         slot.maxLatency,
			Avg:         slot.avgLatency,
			SampleCount: slot.count,
		}
	}
	return stats
}

// GetRegisteredIPs 获取所有注册的IP列表
func (m *LatencyManager) GetRegisteredIPs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ips := make([]string, 0, len(m.slots))
	for ip := range m.slots {
		ips = append(ips, ip)
	}
	return ips
}

// IsRegistered 检查IP是否已注册
func (m *LatencyManager) IsRegistered(ip string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.slots[ip]
	return exists
}
