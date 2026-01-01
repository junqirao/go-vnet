package flow

import (
	"bytes"
	"testing"

	"go-vnet/common/session"
)

// 示例 1: 原地修改中间件（零分配）
// 适用场景：加密、解密、校验和计算等
func inPlaceEncrypt(sess *session.Session, data []byte, buf []byte) []byte {
	for i := range data {
		data[i] ^= 0xAA
	}
	return data
}

// 示例 2: 添加数据头中间件（使用 buf）
// 适用场景：协议封装、添加元数据等
func addHeader(sess *session.Session, data []byte, buf []byte) []byte {
	// 复用 buf，避免分配
	buf = buf[:len(data)+4]
	copy(buf[4:], data)
	buf[0], buf[1], buf[2], buf[3] = 0, 0, 0, 1
	return buf
}

// 示例 3: 添加数据尾中间件（使用 buf）
// 适用场景：添加 CRC、校验码等
func addFooter(sess *session.Session, data []byte, buf []byte) []byte {
	buf = buf[:len(data)+2]
	copy(buf, data)
	buf[len(data)] = 0
	buf[len(data)+1] = 1
	return buf
}

// 示例 4: 翻转第一个字节（原地修改）
func flipFirstByte(sess *session.Session, data []byte, buf []byte) []byte {
	if len(data) > 0 {
		data[0] ^= 0xFF
	}
	return data
}

// 基准测试
func BenchmarkControlRun_1Middleware(b *testing.B) {
	control := NewControl(addHeader)
	data := make([]byte, 1024)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		control.Handle(sess, data)
	}
}

func BenchmarkControlRun_3Middlewares(b *testing.B) {
	control := NewControl(addHeader, addFooter, flipFirstByte)
	data := make([]byte, 1024)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		control.Handle(sess, data)
	}
}

func BenchmarkControlRun_SmallData(b *testing.B) {
	control := NewControl(addHeader, addFooter, flipFirstByte)
	data := make([]byte, 128)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		control.Handle(sess, data)
	}
}

func BenchmarkControlRun_LargeData(b *testing.B) {
	control := NewControl(addHeader, addFooter, flipFirstByte)
	data := make([]byte, 8192)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		control.Handle(sess, data)
	}
}

func BenchmarkControlRun_5Middlewares(b *testing.B) {
	control := NewControl(
		addHeader,
		addFooter,
		flipFirstByte,
		addHeader,
		addFooter,
	)
	data := make([]byte, 1024)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		control.Handle(sess, data)
	}
}

func BenchmarkControlRun_NopControl(b *testing.B) {
	control := NopControl
	data := make([]byte, 1024)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		control.Handle(sess, data)
	}
}

// 并发测试
func BenchmarkControlRun_Parallel(b *testing.B) {
	control := NewControl(addHeader, addFooter, flipFirstByte)
	data := make([]byte, 1024)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			control.Handle(sess, data)
		}
	})
}

// 测试原地操作中间件（零分配）
func BenchmarkControlRun_InPlace(b *testing.B) {
	control := NewControl(inPlaceEncrypt, inPlaceEncrypt, inPlaceEncrypt)
	data := make([]byte, 1024)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		control.Handle(sess, data)
	}
}

// 测试 ZeroAllocRun - 零分配版本（已移除，统一使用 Handle）
// 现在所有场景都使用 Handle 方法，自动实现零分配
func BenchmarkControlRun_ZeroAlloc_Old(b *testing.B) {
	control := NewControl(inPlaceEncrypt, inPlaceEncrypt, inPlaceEncrypt)
	data := make([]byte, 1024)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		control.Handle(sess, data)
	}
}

func BenchmarkControlRun_ZeroAlloc_Parallel_Old(b *testing.B) {
	control := NewControl(inPlaceEncrypt, inPlaceEncrypt, inPlaceEncrypt)
	data := make([]byte, 1024)
	sess := &session.Session{}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			control.Handle(sess, data)
		}
	})
}

// 单元测试
func TestControlRun(t *testing.T) {
	control := NewControl(addHeader, addFooter, flipFirstByte)
	data := []byte{0x01, 0x02, 0x03, 0x04}
	sess := &session.Session{}

	result := control.Handle(sess, data)

	// 验证数据长度
	expectedLen := len(data) + 4 + 2
	if len(result) != expectedLen {
		t.Errorf("expected length %d, got %d", expectedLen, len(result))
	}

	// addHeader 添加前缀 [0 0 0 1] + data
	// addFooter 添加后缀 [0 0 0 1 data data 0 1]
	// flipFirstByte 翻转第一个字节: [0 ^ 0xFF, 0, 0, 1, data, data, 0, 1] = [255, 0, 0, 1, data, data, 0 1]

	// 验证前缀（flipFirstByte 翻转了第一个字节）
	if result[0] != 0xFF || result[1] != 0 || result[2] != 0 || result[3] != 1 {
		t.Errorf("unexpected prefix: %v", result[0:4])
	}

	// 验证原始数据（从索引 4 开始）
	if !bytes.Equal(result[4:4+len(data)], data) {
		t.Errorf("data corrupted: %v", result[4:4+len(data)])
	}

	// 验证后缀
	if result[len(result)-2] != 0 || result[len(result)-1] != 1 {
		t.Errorf("unexpected suffix: %v", result[len(result)-2:])
	}
}

func TestControlRun_Empty(t *testing.T) {
	control := NewControl()
	data := []byte{0x01, 0x02}
	sess := &session.Session{}

	result := control.Handle(sess, data)
	if !bytes.Equal(result, data) {
		t.Errorf("expected %v, got %v", data, result)
	}
}

func TestControlRun_InPlace(t *testing.T) {
	control := NewControl(inPlaceEncrypt)
	data := []byte{0xAA, 0x00, 0x00}
	sess := &session.Session{}

	result := control.Handle(sess, data)

	// 原地异或 0xAA 后，第一个字节应该变为 0
	expected := []byte{0x00, 0xAA, 0xAA}
	if !bytes.Equal(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestControlAddHandler(t *testing.T) {
	control := NewControl()
	if control.Len() != 0 {
		t.Errorf("expected 0 handlers, got %d", control.Len())
	}

	control.AddHandler(addHeader)
	if control.Len() != 1 {
		t.Errorf("expected 1 handler, got %d", control.Len())
	}

	control.AddHandler(addFooter)
	if control.Len() != 2 {
		t.Errorf("expected 2 handlers, got %d", control.Len())
	}

	control.Clear()
	if control.Len() != 0 {
		t.Errorf("expected 0 handlers after clear, got %d", control.Len())
	}
}

func ExampleHandler_inPlace() {
	// 原地修改中间件（零分配）
	encrypt := func(sess *session.Session, data []byte, buf []byte) []byte {
		for i := range data {
			data[i] ^= 0xFF
		}
		return data
	}

	control := NewControl(encrypt)
	data := []byte{0x01, 0x02, 0x03}
	sess := &session.Session{}

	result := control.Handle(sess, data)
	// result: [0xFE, 0xFD, 0xFC]
	_ = result
}

func ExampleHandler_withBuffer() {
	// 使用 buf 的中间件（复用内存）
	addHeader := func(sess *session.Session, data []byte, buf []byte) []byte {
		buf = buf[:len(data)+4]
		copy(buf[4:], data)
		copy(buf[:4], []byte{0, 0, 0, 1})
		return buf
	}

	control := NewControl(addHeader)
	data := []byte{0x01, 0x02}
	sess := &session.Session{}

	result := control.Handle(sess, data)
	// result: [0, 0, 0, 1, 0x01, 0x02]
	_ = result
}
