package metrics

import (
	"strconv"
	"sync/atomic"
	"time"
)

type (
	TransportMetrics struct {
		RxBytes   *BytesCounter  `json:"rx_bytes,omitempty"`
		RxPackets *PacketCounter `json:"rx_packets,omitempty"`
		TxBytes   *BytesCounter  `json:"tx_bytes,omitempty"`
		TxPackets *PacketCounter `json:"tx_packets,omitempty"`
	}
	BytesCounter struct {
		Total atomic.Uint64 // 总字节数
		Last  atomic.Uint64 // 上次上报值
		Speed atomic.Uint64 // 当前速度
	}
	PacketCounter struct {
		Total atomic.Uint64 // 总包数
		Last  atomic.Uint64 // 上次上报值
		Speed atomic.Uint64 // 当前速度
	}
)

// NewTransportMetrics 创建新的传输指标
func NewTransportMetrics() *TransportMetrics {
	return &TransportMetrics{
		RxBytes:   NewBytesCounter(),
		RxPackets: NewPacketCounter(),
		TxBytes:   NewBytesCounter(),
		TxPackets: NewPacketCounter(),
	}
}

// NewBytesCounter 创建字节计数器
func NewBytesCounter() *BytesCounter {
	return &BytesCounter{}
}

// NewPacketCounter 创建包计数器
func NewPacketCounter() *PacketCounter {
	return &PacketCounter{}
}

// Add 增加字节数（原子操作，无锁，极致性能）
func (c *BytesCounter) Add(delta uint64) {
	c.Total.Add(delta)
}

// Get 获取当前统计快照（非侵入式读取）
func (c *BytesCounter) Get() (total, last, speed uint64) {
	total = c.Total.Load()
	last = c.Last.Load()
	speed = c.Speed.Load()
	return
}

// UpdateStats 更新统计信息（用于上报时调用）
func (c *BytesCounter) UpdateStats() {
	total := c.Total.Load()
	last := c.Last.Load()

	// 计算增量
	delta := total - last

	// 更新速度和上次值
	if delta > 0 {
		c.Speed.Store(delta)
	} else {
		c.Speed.Store(0)
	}
	c.Last.Store(total)
}

// Add 增加包数（原子操作，无锁，极致性能）
func (c *PacketCounter) Add(delta uint64) {
	c.Total.Add(delta)
}

// Get 获取当前统计快照（非侵入式读取）
func (c *PacketCounter) Get() (total, last, speed uint64) {
	total = c.Total.Load()
	last = c.Last.Load()
	speed = c.Speed.Load()
	return
}

// UpdateStats 更新统计信息（用于上报时调用）
func (c *PacketCounter) UpdateStats() {
	total := c.Total.Load()
	last := c.Last.Load()

	// 计算增量
	delta := total - last

	// 更新速度和上次值
	if delta > 0 {
		c.Speed.Store(delta)
	} else {
		c.Speed.Store(0)
	}
	c.Last.Store(total)
}

// UpdateAll 更新所有传输指标的统计信息
func (t *TransportMetrics) UpdateAll() {
	t.RxBytes.UpdateStats()
	t.RxPackets.UpdateStats()
	t.TxBytes.UpdateStats()
	t.TxPackets.UpdateStats()
}

// Reset 重置所有计数器
func (t *TransportMetrics) Reset() {
	t.RxBytes.Reset()
	t.RxPackets.Reset()
	t.TxBytes.Reset()
	t.TxPackets.Reset()
}

func (t *TransportMetrics) MarshalJSON() ([]byte, error) {
	var buf []byte
	buf = append(buf, '{')

	// RxBytes
	buf = append(buf, `"rx_bytes_total":`...)
	buf = strconv.AppendUint(buf, t.RxBytes.Total.Load(), 10)
	buf = append(buf, ',')
	buf = append(buf, `"rx_bytes_speed":`...)
	buf = strconv.AppendUint(buf, t.RxBytes.Speed.Load(), 10)
	buf = append(buf, ',')

	// RxPackets
	buf = append(buf, `"rx_packets_total":`...)
	buf = strconv.AppendUint(buf, t.RxPackets.Total.Load(), 10)
	buf = append(buf, ',')
	buf = append(buf, `"rx_packets_speed":`...)
	buf = strconv.AppendUint(buf, t.RxPackets.Speed.Load(), 10)
	buf = append(buf, ',')

	// TxBytes
	buf = append(buf, `"tx_bytes_total":`...)
	buf = strconv.AppendUint(buf, t.TxBytes.Total.Load(), 10)
	buf = append(buf, ',')
	buf = append(buf, `"tx_bytes_speed":`...)
	buf = strconv.AppendUint(buf, t.TxBytes.Speed.Load(), 10)
	buf = append(buf, ',')

	// TxPackets
	buf = append(buf, `"tx_packets_total":`...)
	buf = strconv.AppendUint(buf, t.TxPackets.Total.Load(), 10)
	buf = append(buf, ',')
	buf = append(buf, `"tx_packets_speed":`...)
	buf = strconv.AppendUint(buf, t.TxPackets.Speed.Load(), 10)

	buf = append(buf, '}')

	return buf, nil
}

// Reset 重置字节计数器
func (c *BytesCounter) Reset() {
	c.Total.Store(0)
	c.Last.Store(0)
	c.Speed.Store(0)
}

// Reset 重置包计数器
func (c *PacketCounter) Reset() {
	c.Total.Store(0)
	c.Last.Store(0)
	c.Speed.Store(0)
}

// SpeedCalculator 速度计算器，支持基于时间间隔的速度计算
type SpeedCalculator struct {
	lastTime  atomic.Int64
	lastValue atomic.Uint64
}

// NewSpeedCalculator 创建速度计算器
func NewSpeedCalculator() *SpeedCalculator {
	return &SpeedCalculator{}
}

// Calculate 计算速度（bytes/sec 或 packets/sec）
func (s *SpeedCalculator) Calculate(currentValue uint64) uint64 {
	now := time.Now().UnixNano()
	lastTime := s.lastTime.Load()
	lastValue := s.lastValue.Load()

	// 首次调用
	if lastTime == 0 {
		s.lastTime.Store(now)
		s.lastValue.Store(currentValue)
		return 0
	}

	// 计算时间差（纳秒转为秒）
	elapsedNs := now - lastTime
	if elapsedNs <= 0 {
		return 0
	}
	elapsedSec := float64(elapsedNs) / float64(time.Second)

	// 计算增量
	delta := currentValue - lastValue
	if delta == 0 {
		return 0
	}

	// 更新快照
	s.lastTime.Store(now)
	s.lastValue.Store(currentValue)

	// 计算速度
	return uint64(float64(delta) / elapsedSec)
}
