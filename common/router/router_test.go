package router

import (
	"encoding/gob"
	"fmt"
	"strings"
	"testing"
)

func init() {
	// 注册 gob 解码所需的类型
	gob.Register(string(""))
	gob.Register(0)
	gob.Register(int(0))
}

// 测试 Dump 后的大小
func TestRouter_DumpSize(t *testing.T) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRouter()

			// 注册指定数量的路由
			for i := 0; i < tc.routeCount; i++ {
				cidr := fmt.Sprintf("192.168.%d.0/24", i)
				conn := fmt.Sprintf("conn-%d", i)
				err := r.Register(cidr, conn)
				if err != nil {
					t.Fatalf("Register failed for %s: %v", cidr, err)
				}
			}

			// Dump 路由表
			data := r.Dump()
			size := len(data)

			t.Logf("路由数量: %d, Dump后大小: %d bytes (%.2f KB)", tc.routeCount, size, float64(size)/1024)
		})
	}
}

// 测试从 Dump 数据恢复路由表
func TestRouter_RestoreFromDump(t *testing.T) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建原始路由器并注册路由
			original := NewRouter()
			routes := make(map[string]string)
			for i := 0; i < tc.routeCount; i++ {
				cidr := fmt.Sprintf("192.168.%d.0/24", i)
				conn := fmt.Sprintf("conn-%d", i)
				routes[cidr] = conn
				err := original.Register(cidr, conn)
				if err != nil {
					t.Fatalf("Register failed for %s: %v", cidr, err)
				}
			}

			// Dump 路由表
			data := original.Dump()
			t.Logf("Dump 数据大小: %d bytes", len(data))

			// 从 Dump 数据恢复路由器
			restored := NewRouter(data)

			// 验证恢复的路由表
			for cidr, expectedConn := range routes {
				// 提取 IP 进行测试 (去掉掩码)
				ip := cidr[:strings.Index(cidr, "/")]
				v, ok := restored.RouteString(ip)
				if !ok {
					t.Errorf("RouteString failed for %s", ip)
					continue
				}
				if v != expectedConn {
					t.Errorf("Expected %s, got %v for %s", expectedConn, v, ip)
				}
			}
			t.Logf("成功恢复 %d 条路由", tc.routeCount)
		})
	}
}

// 测试 Dump 和 Restore 的压力测试 - 耗时
func BenchmarkRouter_DumpAndRestore(b *testing.B) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// 预先生成路由数据
			original := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				cidr := fmt.Sprintf("192.168.%d.0/24", i)
				conn := fmt.Sprintf("conn-%d", i)
				original.Register(cidr, conn)
			}
			dumpData := original.Dump()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// 从 Dump 数据恢复路由器
				_ = NewRouter(dumpData)
			}
		})
	}
}

// 测试 Restore 方法
func TestRouter_Restore(t *testing.T) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建原始路由器并注册路由
			r := NewRouter()
			routes := make(map[string]string)
			for i := 0; i < tc.routeCount; i++ {
				cidr := fmt.Sprintf("192.168.%d.0/24", i)
				conn := fmt.Sprintf("conn-%d", i)
				routes[cidr] = conn
				err := r.Register(cidr, conn)
				if err != nil {
					t.Fatalf("Register failed for %s: %v", cidr, err)
				}
			}

			// Dump 路由表
			data := r.Dump()
			t.Logf("Dump 数据大小: %d bytes", len(data))

			// 使用 Restore 方法恢复路由表
			err := r.Restore(data)
			if err != nil {
				t.Fatalf("Restore failed: %v", err)
			}

			// 验证恢复的路由表
			for cidr, expectedConn := range routes {
				// 提取 IP 进行测试 (去掉掩码)
				ip := cidr[:strings.Index(cidr, "/")]
				v, ok := r.RouteString(ip)
				if !ok {
					t.Errorf("RouteString failed for %s", ip)
					continue
				}
				if v != expectedConn {
					t.Errorf("Expected %s, got %v for %s", expectedConn, v, ip)
				}
			}
			t.Logf("成功恢复 %d 条路由", tc.routeCount)
		})
	}
}

// 测试 Restore 空数据
func TestRouter_Restore_Empty(t *testing.T) {
	r := NewRouter()
	err := r.Restore(nil)
	if err != nil {
		t.Fatalf("Restore with nil data failed: %v", err)
	}
	err = r.Restore([]byte{})
	if err != nil {
		t.Fatalf("Restore with empty data failed: %v", err)
	}
}

// 测试 Restore 基准测试
func BenchmarkRouter_Restore(b *testing.B) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// 预先生成路由数据
			original := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				cidr := fmt.Sprintf("192.168.%d.0/24", i)
				conn := fmt.Sprintf("conn-%d", i)
				original.Register(cidr, conn)
			}
			dumpData := original.Dump()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				r := NewRouter()
				_ = r.Restore(dumpData)
			}
		})
	}
}

// 测试 Dump 耗时
func BenchmarkRouter_Dump(b *testing.B) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			r := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				cidr := fmt.Sprintf("192.168.%d.0/24", i)
				conn := fmt.Sprintf("conn-%d", i)
				r.Register(cidr, conn)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = r.Dump()
			}
		})
	}
}

func TestRouteTable_Lookup(t *testing.T) {
	table := NewRouteTable()
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(fmt.Sprintf("192.168.%v.0/24", i), "123")
	}
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(fmt.Sprintf("192.168.1.%v/32", i), i)
	}
	res, ok := table.Lookup("192.168.1.55")
	if !ok {
		t.Fatal("lookup failed")
		return
	}
	if res != 55 {
		t.Fatal("lookup failed")
		return
	}
}

func BenchmarkRouteTable_Lookup(b *testing.B) {
	table := NewRouteTable()
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(fmt.Sprintf("192.168.%v.0/24", i), "123")
	}
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(fmt.Sprintf("192.168.1.%v/32", i), i)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		table.Lookup("192.168.1.55")
	}
}
