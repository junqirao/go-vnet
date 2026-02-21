package hub

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Ping 协议包格式
// +--------+--------+--------+----------+----------+--------+
// | Magic  | Type   | SeqNo  | Timestamp| Reserved | Data   |
// | 4 bytes| 1 byte | 4 bytes| 8 bytes  | 4 bytes  | N      |
// +--------+--------+--------+----------+----------+--------+

const (
	// PingMagic ping魔数标识
	PingMagic = "VNP1"

	// PingHeaderLen ping头部固定长度
	PingHeaderLen = 4 + 1 + 4 + 8 + 4

	// MaxPingPacketSize 最大ping包大小
	MaxPingPacketSize = 1400
)

var (
	// PingMagicBytes ping魔数字节
	PingMagicBytes = []byte(PingMagic)

	// ErrInvalidPingPacket 无效的ping包
	ErrInvalidPingPacket = fmt.Errorf("invalid ping packet")
)

// PingType ping类型
type PingType byte

const (
	// PingTypeRequest ping请求
	PingTypeRequest PingType = 0x01
	// PingTypeResponse ping响应
	PingTypeResponse PingType = 0x02
)

// PingPacket ping包
type PingPacket struct {
	Magic     string
	Type      PingType
	SeqNo     uint32
	Timestamp int64
	Reserved  uint32
	Data      []byte
}

// NewPingPacket 创建ping包
func NewPingPacket(typ PingType, seqNo uint32, timestamp int64, data []byte) *PingPacket {
	return &PingPacket{
		Magic:     PingMagic,
		Type:      typ,
		SeqNo:     seqNo,
		Timestamp: timestamp,
		Reserved:  0,
		Data:      data,
	}
}

// NewPingRequest 创建ping请求包
func NewPingRequest(seqNo uint32, data []byte) *PingPacket {
	return NewPingPacket(PingTypeRequest, seqNo, time.Now().UnixNano(), data)
}

// NewPingResponse 创建ping响应包
func NewPingResponse(req *PingPacket) *PingPacket {
	return NewPingPacket(PingTypeResponse, req.SeqNo, req.Timestamp, req.Data)
}

// Encode 编码ping包
func (p *PingPacket) Encode() []byte {
	totalLen := PingHeaderLen + len(p.Data)
	if totalLen > MaxPingPacketSize {
		totalLen = MaxPingPacketSize
	}

	buf := make([]byte, totalLen)
	off := 0

	// Magic (4 bytes)
	copy(buf[off:off+4], PingMagicBytes)
	off += 4

	// Type (1 byte)
	buf[off] = byte(p.Type)
	off += 1

	// SeqNo (4 bytes)
	binary.BigEndian.PutUint32(buf[off:off+4], p.SeqNo)
	off += 4

	// Timestamp (8 bytes)
	binary.BigEndian.PutUint64(buf[off:off+8], uint64(p.Timestamp))
	off += 8

	// Reserved (4 bytes)
	binary.BigEndian.PutUint32(buf[off:off+4], p.Reserved)
	off += 4

	// Data
	if len(p.Data) > 0 {
		copy(buf[off:], p.Data)
	}

	return buf
}

// DecodePingPacket 解码ping包
func DecodePingPacket(buf []byte) (*PingPacket, error) {
	if len(buf) < PingHeaderLen {
		return nil, ErrInvalidPingPacket
	}

	off := 0

	// Magic (4 bytes)
	magic := string(buf[off : off+4])
	if magic != PingMagic {
		return nil, ErrInvalidPingPacket
	}
	off += 4

	p := &PingPacket{
		Magic: magic,
	}

	// Type (1 byte)
	p.Type = PingType(buf[off])
	off += 1

	// SeqNo (4 bytes)
	p.SeqNo = binary.BigEndian.Uint32(buf[off : off+4])
	off += 4

	// Timestamp (8 bytes)
	p.Timestamp = int64(binary.BigEndian.Uint64(buf[off : off+8]))
	off += 8

	// Reserved (4 bytes)
	p.Reserved = binary.BigEndian.Uint32(buf[off : off+4])
	off += 4

	// Data
	if len(buf) > off {
		p.Data = make([]byte, len(buf)-off)
		copy(p.Data, buf[off:])
	}

	return p, nil
}

// IsRequest 判断是否为请求包
func (p *PingPacket) IsRequest() bool {
	return p.Type == PingTypeRequest
}

// IsResponse 判断是否为响应包
func (p *PingPacket) IsResponse() bool {
	return p.Type == PingTypeResponse
}

// PingClient ping客户端
type PingClient struct {
	mu           sync.Mutex
	conn         *net.UDPConn
	targetAddr   *net.UDPAddr
	seqNo        uint32
	timeout      time.Duration
	responseChan chan *PingPacket
	stopped      bool
}

// NewPingClient 创建ping客户端
func NewPingClient(ctx context.Context, targetAddr string, timeout time.Duration) (*PingClient, error) {
	// 解析目标地址
	udpAddr, err := net.ResolveUDPAddr("udp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve target address failed: %w", err)
	}

	// 创建UDP连接（随机本地端口）
	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, fmt.Errorf("listen udp failed: %w", err)
	}

	c := &PingClient{
		conn:         conn,
		targetAddr:   udpAddr,
		seqNo:        0,
		timeout:      timeout,
		responseChan: make(chan *PingPacket, 100),
		stopped:      false,
	}

	// 启动接收协程
	go c.receiveLoop(ctx)

	g.Log().Infof(ctx, "[PING] client started: local=%s, target=%s",
		conn.LocalAddr(), targetAddr)

	return c, nil
}

// Ping 发送ping并返回RTT
func (c *PingClient) Ping(ctx context.Context, data []byte) (time.Duration, error) {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return 0, fmt.Errorf("client is stopped")
	}
	c.seqNo++
	seqNo := c.seqNo
	c.mu.Unlock()

	// 创建ping请求
	req := NewPingRequest(seqNo, data)
	reqBuf := req.Encode()

	// 发送ping请求
	startTime := time.Now()
	_, err := c.conn.WriteToUDP(reqBuf, c.targetAddr)
	if err != nil {
		return 0, fmt.Errorf("send ping failed: %w", err)
	}

	// g.Log().Debugf(ctx, "[PING] sent ping request: seq=%d, target=%s", seqNo, c.targetAddr)

	// 等待响应或超时
	timeout := time.NewTimer(c.timeout)
	defer timeout.Stop()

	select {
	case resp, ok := <-c.responseChan:
		if !ok || resp == nil {
			return 0, fmt.Errorf("response channel closed")
		}
		if resp.SeqNo != seqNo {
			return 0, fmt.Errorf("ping response seq mismatch: got %d, want %d",
				resp.SeqNo, seqNo)
		}
		rtt := time.Since(startTime)
		// g.Log().Debugf(ctx, "[PING] received ping response: seq=%d, rtt=%v",
		// 	seqNo, rtt)
		return rtt, nil

	case <-timeout.C:
		return 0, fmt.Errorf("ping timeout: seq=%d, timeout=%v", seqNo, c.timeout)

	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// receiveLoop 接收ping响应
func (c *PingClient) receiveLoop(ctx context.Context) {
	buf := make([]byte, MaxPingPacketSize)

	for {
		// 检查是否已停止
		c.mu.Lock()
		stopped := c.stopped
		c.mu.Unlock()
		if stopped {
			return
		}

		select {
		case <-ctx.Done():
			return

		default:
			// 设置读取超时
			_ = c.conn.SetReadDeadline(time.Now().Add(time.Second))

			n, addr, err := c.conn.ReadFromUDP(buf)
			if err != nil {
				// 检查是否已停止
				c.mu.Lock()
				stopped := c.stopped
				c.mu.Unlock()
				if stopped {
					return
				}

				if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
					continue
				}
				// 检查是否是连接关闭错误
				if err.Error() == "use of closed network connection" {
					// 连接已关闭，退出循环
					return
				}
				// 其他错误也退出，避免无限循环
				g.Log().Debugf(ctx, "[PING] receive error: %v, exiting", err)
				return
			}

			// 解码ping包
			packet, err := DecodePingPacket(buf[:n])
			if err != nil {
				g.Log().Debugf(ctx, "[PING] decode packet failed from %s: %v", addr, err)
				continue
			}

			// 只处理响应包
			if !packet.IsResponse() {
				continue
			}

			// 发送到响应通道
			select {
			case c.responseChan <- packet:
			default:
				g.Log().Debugf(ctx, "[PING] response channel full, dropped response: seq=%d", packet.SeqNo)
			}
		}
	}
}

// Close 关闭ping客户端
func (c *PingClient) Close() error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	c.stopped = true
	c.mu.Unlock()

	close(c.responseChan)

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// PingServer ping服务器
type PingServer struct {
	mu      sync.RWMutex
	conn    *net.UDPConn
	stopped bool
}

// NewPingServer 创建ping服务器
func NewPingServer(ctx context.Context, listenAddr string) (*PingServer, error) {
	// 解析监听地址
	addr, err := net.ResolveUDPAddr("udp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve address failed: %w", err)
	}

	// 创建UDP连接
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen udp failed: %w", err)
	}

	s := &PingServer{
		conn:    conn,
		stopped: false,
	}

	// 启动接收协程
	go s.receiveLoop(ctx)

	g.Log().Infof(ctx, "[PING] server started: listen=%s", conn.LocalAddr())

	return s, nil
}

// receiveLoop 接收并处理ping请求
func (s *PingServer) receiveLoop(ctx context.Context) {
	buf := make([]byte, MaxPingPacketSize)

	for {
		// 检查是否已停止
		s.mu.Lock()
		stopped := s.stopped
		s.mu.Unlock()
		if stopped {
			return
		}

		select {
		case <-ctx.Done():
			return

		default:
			// 设置读取超时
			_ = s.conn.SetReadDeadline(time.Now().Add(time.Second))

			n, addr, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				// 再次检查是否已停止
				s.mu.Lock()
				stopped := s.stopped
				s.mu.Unlock()
				if stopped {
					return
				}

				if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
					continue
				}
				// 检查是否是连接关闭错误
				if err.Error() == "use of closed network connection" {
					// 连接已关闭，静默退出
					return
				}
				// 其他错误记录并退出，避免无限循环
				g.Log().Debugf(ctx, "[PING] receive error: %v, exiting", err)
				return
			}

			// 解码ping包
			packet, err := DecodePingPacket(buf[:n])
			if err != nil {
				g.Log().Warningf(ctx, "[PING] decode packet failed from %s: %v", addr, err)
				continue
			}

			// 只处理请求包
			if !packet.IsRequest() {
				continue
			}

			// g.Log().Debugf(ctx, "[PING] server received ping request: seq=%d, addr=%s, dataLen=%d",
			// 	packet.SeqNo, addr, len(packet.Data))

			// 立即发送响应
			resp := NewPingResponse(packet)
			respBuf := resp.Encode()

			_, err = s.conn.WriteToUDP(respBuf, addr)
			if err != nil {
				g.Log().Warningf(ctx, "[PING] send response failed to %s: %v", addr, err)
				continue
			}
			// g.Log().Debugf(ctx, "[PING] sent ping response: seq=%d, addr=%s",
			// 	resp.SeqNo, addr)
		}
	}
}

// Close 关闭ping服务器
func (s *PingServer) Close() error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.mu.Unlock()

	if s.conn != nil {
		return s.conn.Close()
	}

	return nil
}

// Addr 获取监听地址
func (s *PingServer) Addr() net.Addr {
	if s.conn != nil {
		return s.conn.LocalAddr()
	}
	return nil
}

// LatencyStats 延迟统计信息
// 存储目标IP到延迟统计的映射，供上层查询使用
type LatencyStats struct {
	Current     time.Duration `json:"current"`
	Min         time.Duration `json:"min"`
	Max         time.Duration `json:"max"`
	Avg         time.Duration `json:"avg"`
	SampleCount int           `json:"sample_count"`
}

func (l LatencyStats) MarshalJSON() ([]byte, error) {
	var b strings.Builder
	b.Grow(100) // 预分配足够空间: {"current":"XXX.XX","min":"XXX.XX","max":"XXX.XX","avg":"XXX.XX","sample_count":X}

	b.WriteString(`{"current":"`)
	b.WriteString(formatDuration(l.Current))
	b.WriteString(`","min":"`)
	b.WriteString(formatDuration(l.Min))
	b.WriteString(`","max":"`)
	b.WriteString(formatDuration(l.Max))
	b.WriteString(`","avg":"`)
	b.WriteString(formatDuration(l.Avg))
	b.WriteString(`","sample_count":`)
	b.WriteString(strconv.Itoa(l.SampleCount))
	b.WriteByte('}')

	return []byte(b.String()), nil
}

// formatDuration 格式化时间持续时间为毫秒字符串（保留2位小数）
func formatDuration(d time.Duration) string {
	ms := float64(d.Nanoseconds()) / 1e6
	return strconv.FormatFloat(ms, 'f', 2, 64)
}
