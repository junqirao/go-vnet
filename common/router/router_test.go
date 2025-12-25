package router

import (
	"fmt"
	"strings"
	"testing"
)

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
		{"512 routes", 512},
		{"1K routes", 1 * 1024},
		{"6K routes", 6 * 1024},
		{"10K routes", 10 * 1024},
		{"60K routes", 60 * 1024},
		{"100K routes", 100 * 1024},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewRouter()

			// 注册指定数量的路由
			for i := 0; i < tc.routeCount; i++ {
				// 使用多个IP段来支持大量路由
				octet1 := 10 + i/65536
				octet2 := (i / 256) % 256
				octet3 := i % 256
				cidr := fmt.Sprintf("%d.%d.%d.0/24", octet1, octet2, octet3)
				conn := fmt.Sprintf("conn-%d", i)
				err := r.Register(cidr, conn)
				if err != nil {
					t.Fatalf("Register failed for %s: %v", cidr, err)
				}
			}

			// Dump 路由表
			data := r.Dump()
			size := len(data)

			t.Logf("路由数量: %d, Dump后大小: %d bytes (%.2f KB, %.2f MB)", tc.routeCount, size, float64(size)/1024, float64(size)/1024/1024)
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
		{"512 routes", 512},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建原始路由器并注册路由
			original := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.168.%d.0/24", octet1, octet2)
				conn := fmt.Sprintf("conn-%d", i)
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

			// 验证恢复的路由表结构（target为nil是正常的）
			for i := 0; i < tc.routeCount; i++ {
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.168.%d.0/24", octet1, octet2)
				// 提取 IP 进行测试 (去掉掩码)
				ip := cidr[:strings.Index(cidr, "/")]
				v, ok := restored.RouteString(ip)
				// 由于target不序列化，target应该为nil，但isLeaf应该为true
				if !ok {
					t.Errorf("RouteString failed for %s", ip)
					continue
				}
				// target应该是nil，因为只同步结构
				if v != nil {
					t.Logf("Warning: target is not nil for %s (expected nil, got %v)", ip, v)
				}
			}
			t.Logf("成功恢复 %d 条路由结构", tc.routeCount)
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
		{"512 routes", 512},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// 预先生成路由数据
			original := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.168.%d.0/24", octet1, octet2)
				conn := fmt.Sprintf("conn-%d", i)
				_ = original.Register(cidr, conn)
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

// 测试 Restore 方法（合并模式，不覆盖已有路由）
func TestRouter_Restore(t *testing.T) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
		{"512 routes", 512},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建原始路由器并注册路由
			r := NewRouter()
			routes := make(map[string]string)
			for i := 0; i < tc.routeCount; i++ {
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.168.%d.0/24", octet1, octet2)
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

			// 使用 Restore 方法恢复路由表（合并模式）
			// 由于是合并到同一个路由表，原有的target应该保持不变
			err := r.Restore(data)
			if err != nil {
				t.Fatalf("Restore failed: %v", err)
			}

			// 验证恢复的路由表（target应该保持不变）
			for cidr, expectedConn := range routes {
				// 提取 IP 进行测试 (去掉掩码)
				ip := cidr[:strings.Index(cidr, "/")]
				v, ok := r.RouteString(ip)
				if !ok {
					t.Errorf("RouteString failed for %s", ip)
					continue
				}
				// 在合并模式下，原有的target应该保持不变
				if v != expectedConn {
					t.Errorf("Expected %s, got %v for %s", expectedConn, v, ip)
				}
			}
			t.Logf("成功恢复 %d 条路由，原有target保持不变", tc.routeCount)
		})
	}
}

// 测试 Restore 合并模式：不覆盖已有路由
func TestRouter_Restore_Merge(t *testing.T) {
	// 创建两个路由器，分别注册不同的路由
	router1 := NewRouter()
	router2 := NewRouter()

	// router1 注册路由1
	err := router1.Register("10.0.1.0/24", "target-1")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	err = router1.Register("10.0.2.0/24", "target-2")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// router2 注册路由2
	err = router2.Register("10.0.2.0/24", "target-2-new")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	err = router2.Register("10.0.3.0/24", "target-3")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Dump router2 的数据
	data := router2.Dump()

	// 将 router2 的数据合并到 router1
	err = router1.Restore(data)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 验证路由1: 应该保持原有target
	v, ok := router1.RouteString("10.0.1.1")
	if !ok {
		t.Errorf("RouteString failed for 10.0.1.1")
	} else if v != "target-1" {
		t.Errorf("Expected target-1, got %v for 10.0.1.1", v)
	}

	// 验证路由2: 应该保持原有target（不被覆盖）
	v, ok = router1.RouteString("10.0.2.1")
	if !ok {
		t.Errorf("RouteString failed for 10.0.2.1")
	} else if v != "target-2" {
		t.Errorf("Expected target-2 (not overridden), got %v for 10.0.2.1", v)
	}

	// 验证路由3: 应该新增（从router2获取）
	// 注意：由于只同步结构不同步target，target应该是nil
	v, ok = router1.RouteString("10.0.3.1")
	if !ok {
		t.Errorf("RouteString failed for 10.0.3.1")
	}
	// target应该为nil，因为只同步结构
	if v != nil {
		t.Logf("Note: new route 10.0.3.1 has target %v (nil expected, but non-nil is acceptable)", v)
	}
}

// 测试 Restore 空数据
func TestRouter_Restore_Empty(t *testing.T) {
	r := NewRouter()
	// 先注册一些路由
	err := r.Register("10.0.1.0/24", "target-1")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err = r.Restore(nil)
	if err != nil {
		t.Fatalf("Restore with nil data failed: %v", err)
	}
	// 验证原有路由仍然存在
	v, ok := r.RouteString("10.0.1.1")
	if !ok || v != "target-1" {
		t.Error("Existing route lost after Restore with nil")
	}

	err = r.Restore([]byte{})
	if err != nil {
		t.Fatalf("Restore with empty data failed: %v", err)
	}
	// 验证原有路由仍然存在
	v, ok = r.RouteString("10.0.1.1")
	if !ok || v != "target-1" {
		t.Error("Existing route lost after Restore with empty data")
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
		{"512 routes", 512},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			// 预先生成路由数据
			original := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.168.%d.0/24", octet1, octet2)
				conn := fmt.Sprintf("conn-%d", i)
				_ = original.Register(cidr, conn)
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
		{"512 routes", 512},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			r := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.168.%d.0/24", octet1, octet2)
				conn := fmt.Sprintf("conn-%d", i)
				_ = r.Register(cidr, conn)
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

// 测试路由表Hash功能
func TestRouter_Hash(t *testing.T) {
	r1 := NewRouter()
	r2 := NewRouter()

	// 添加相同的路由
	for i := 0; i < 100; i++ {
		cidr := fmt.Sprintf("192.168.%d.0/24", i)
		conn := fmt.Sprintf("conn-%d", i)
		_ = r1.Register(cidr, conn)
		_ = r2.Register(cidr, conn)
	}

	// 相同的路由表应该有相同的Hash
	hash1 := r1.Hash()
	hash2 := r2.Hash()
	if hash1 != hash2 {
		t.Errorf("相同路由表的Hash不同: %s vs %s", hash1, hash2)
	}

	// 添加一条新路由到r2
	_ = r2.Register("10.0.1.0/24", "new-conn")

	// 不同的路由表应该有不同的Hash
	hash3 := r2.Hash()
	if hash1 == hash3 {
		t.Error("不同路由表的Hash相同，应该不同")
	}

	t.Logf("Hash1: %s", hash1)
	t.Logf("Hash2: %s", hash2)
	t.Logf("Hash3 (modified): %s", hash3)
}

// 测试Hash性能
func BenchmarkRouter_Hash(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		cidr := fmt.Sprintf("192.168.%d.0/24", i%256)
		conn := fmt.Sprintf("conn-%d", i)
		_ = r.Register(cidr, conn)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = r.Hash()
	}
}

// 测试Hash性能 - 不同路由数量的基准测试
func BenchmarkRouter_Hash_Scales(b *testing.B) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
		{"512 routes", 512},
		{"1024 routes", 1024},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			r := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.168.%d.0/24", octet1, octet2)
				conn := fmt.Sprintf("conn-%d", i)
				_ = r.Register(cidr, conn)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = r.Hash()
			}
		})
	}
}
