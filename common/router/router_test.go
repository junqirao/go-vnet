package router

import (
	"encoding/base64"
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
				err := r.Register(nil, cidr, conn)
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
				err := original.Register(nil, cidr, conn)
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
				v, ok := restored.Route(ip)
				// 由于target不序列化，target应该为nil，但isLeaf应该为true
				if !ok {
					t.Errorf("Route failed for %s", ip)
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
				_ = original.Register(nil, cidr, conn)
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
				err := r.Register(nil, cidr, conn)
				if err != nil {
					t.Fatalf("Register failed for %s: %v", cidr, err)
				}
			}

			// Dump 路由表
			data := r.Dump()
			t.Logf("Dump 数据大小: %d bytes", len(data))

			// 使用 Restore 方法恢复路由表（合并模式）
			// 由于是合并到同一个路由表，原有的target应该保持不变
			err := r.Restore(nil, data)
			if err != nil {
				t.Fatalf("Restore failed: %v", err)
			}

			// 验证恢复的路由表（target应该保持不变）
			for cidr, expectedConn := range routes {
				// 提取 IP 进行测试 (去掉掩码)
				ip := cidr[:strings.Index(cidr, "/")]
				v, ok := r.Route(ip)
				if !ok {
					t.Errorf("Route failed for %s", ip)
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
	err := router1.Register(nil, "10.0.1.0/24", "target-1")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	err = router1.Register(nil, "10.0.2.0/24", "target-2")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// router2 注册路由2
	err = router2.Register(nil, "10.0.2.0/24", "target-2-new")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	err = router2.Register(nil, "10.0.3.0/24", "target-3")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Dump router2 的数据
	data := router2.Dump()

	// 将 router2 的数据合并到 router1
	err = router1.Restore(nil, data)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// 验证路由1: 应该保持原有target
	v, ok := router1.Route("10.0.1.1")
	if !ok {
		t.Errorf("Route failed for 10.0.1.1")
	} else if v != "target-1" {
		t.Errorf("Expected target-1, got %v for 10.0.1.1", v)
	}

	// 验证路由2: 应该保持原有target（不被覆盖）
	v, ok = router1.Route("10.0.2.1")
	if !ok {
		t.Errorf("Route failed for 10.0.2.1")
	} else if v != "target-2" {
		t.Errorf("Expected target-2 (not overridden), got %v for 10.0.2.1", v)
	}

	// 验证路由3: 应该新增（从router2获取）
	// 注意：由于只同步结构不同步target，target应该是nil
	v, ok = router1.Route("10.0.3.1")
	if !ok {
		t.Errorf("Route failed for 10.0.3.1")
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
	err := r.Register(nil, "10.0.1.0/24", "target-1")
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err = r.Restore(nil, nil)
	if err != nil {
		t.Fatalf("Restore with nil data failed: %v", err)
	}
	// 验证原有路由仍然存在
	v, ok := r.Route("10.0.1.1")
	if !ok || v != "target-1" {
		t.Error("Existing route lost after Restore with nil")
	}

	err = r.Restore(nil, []byte{})
	if err != nil {
		t.Fatalf("Restore with empty data failed: %v", err)
	}
	// 验证原有路由仍然存在
	v, ok = r.Route("10.0.1.1")
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
				_ = original.Register(nil, cidr, conn)
			}
			dumpData := original.Dump()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				r := NewRouter()
				_ = r.Restore(nil, dumpData)
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
				_ = r.Register(nil, cidr, conn)
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
		_ = table.AddRoute(nil, fmt.Sprintf("192.168.%v.0/24", i), "123")
	}
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(nil, fmt.Sprintf("192.168.1.%v/32", i), i)
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
		_ = table.AddRoute(nil, fmt.Sprintf("192.168.%v.0/24", i), "123")
	}
	for i := 0; i < 255; i++ {
		_ = table.AddRoute(nil, fmt.Sprintf("192.168.1.%v/32", i), i)
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
		_ = r1.Register(nil, cidr, conn)
		_ = r2.Register(nil, cidr, conn)
	}

	// 相同的路由表应该有相同的Hash
	hash1 := r1.Hash()
	hash2 := r2.Hash()
	if hash1 != hash2 {
		t.Errorf("相同路由表的Hash不同: %s vs %s", hash1, hash2)
	}

	// 添加一条新路由到r2
	_ = r2.Register(nil, "10.0.1.0/24", "new-conn")

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
		_ = r.Register(nil, cidr, conn)
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
				_ = r.Register(nil, cidr, conn)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = r.Hash()
			}
		})
	}
}

// 测试Dump缓存机制
func TestRouter_DumpCache(t *testing.T) {
	r := NewRouter()

	// 注册一些路由
	for i := 0; i < 100; i++ {
		cidr := fmt.Sprintf("192.168.%d.0/24", i)
		conn := fmt.Sprintf("conn-%d", i)
		_ = r.Register(nil, cidr, conn)
	}

	// 第一次Dump，应该生成缓存
	data1 := r.Dump()
	t.Logf("第一次Dump: %d bytes", len(data1))

	// 第二次Dump，应该返回缓存（相同的数据）
	data2 := r.Dump()
	t.Logf("第二次Dump: %d bytes", len(data2))

	// 验证两次Dump的数据相同
	if string(data1) != string(data2) {
		t.Error("两次Dump的数据应该相同")
	}

	// 修改路由表
	_ = r.Register(nil, "10.0.1.0/24", "new-conn")

	// 第三次Dump，应该生成新的缓存
	data3 := r.Dump()
	t.Logf("第三次Dump（修改后）: %d bytes", len(data3))

	// 验证修改后的数据与之前不同
	if string(data1) == string(data3) {
		t.Error("修改路由表后，Dump数据应该不同")
	}

	// 第四次Dump，应该返回新的缓存
	data4 := r.Dump()
	t.Logf("第四次Dump: %d bytes", len(data4))

	// 验证返回的是同一个slice（新缓存）
	if string(data3) != string(data4) {
		t.Error("第四次Dump的数据应该与第三次相同（使用缓存）")
	}
}

// 测试Dump缓存的并发安全性
func TestRouter_DumpCache_Concurrent(t *testing.T) {
	r := NewRouter()

	// 注册一些路由
	for i := 0; i < 100; i++ {
		cidr := fmt.Sprintf("192.168.%d.0/24", i)
		conn := fmt.Sprintf("conn-%d", i)
		_ = r.Register(nil, cidr, conn)
	}

	// 并发测试
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			data := r.Dump()
			if len(data) == 0 {
				t.Error("Dump返回空数据")
			}
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 100; i++ {
		<-done
	}
}

// 测试Dump缓存的性能
func BenchmarkRouter_Dump_WithCache(b *testing.B) {
	r := NewRouter()

	// 预先生成路由表
	for i := 0; i < 1000; i++ {
		cidr := fmt.Sprintf("192.168.%d.0/24", i%256)
		conn := fmt.Sprintf("conn-%d", i)
		_ = r.Register(nil, cidr, conn)
	}

	// 预热：生成缓存
	_ = r.Dump()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = r.Dump()
	}
}

// 测试Len方法 - 空路由表
func TestRouter_Len_Empty(t *testing.T) {
	r := NewRouter()
	if r.Len() != 0 {
		t.Errorf("Expected empty router length to be 0, got %d", r.Len())
	}
}

// 测试Len方法 - 单条路由
func TestRouter_Len_Single(t *testing.T) {
	r := NewRouter()
	_ = r.Register(nil, "192.168.1.0/24", "target-1")
	if r.Len() != 1 {
		t.Errorf("Expected router length to be 1, got %d", r.Len())
	}
}

// 测试Len方法 - 多条路由
func TestRouter_Len_Multiple(t *testing.T) {
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
			r := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				// 使用多个IP段来支持大量路由
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.%d.0.0/16", octet1, octet2)
				conn := fmt.Sprintf("conn-%d", i)
				_ = r.Register(nil, cidr, conn)
			}
			if r.Len() != tc.routeCount {
				t.Errorf("Expected router length to be %d, got %d", tc.routeCount, r.Len())
			}
		})
	}
}

// 测试Len方法 - AddRoute后增加
func TestRouter_Len_AddRoute(t *testing.T) {
	r := NewRouter()
	_ = r.Register(nil, "192.168.1.0/24", "target-1")
	if r.Len() != 1 {
		t.Errorf("Expected router length to be 1, got %d", r.Len())
	}

	_ = r.Register(nil, "192.168.2.0/24", "target-2")
	if r.Len() != 2 {
		t.Errorf("Expected router length to be 2, got %d", r.Len())
	}
}

// 测试Len方法 - DeleteRoute后减少
func TestRouter_Len_DeleteRoute(t *testing.T) {
	r := NewRouter()
	_ = r.Register(nil, "192.168.1.0/24", "target-1")
	_ = r.Register(nil, "192.168.2.0/24", "target-2")
	_ = r.Register(nil, "192.168.3.0/24", "target-3")

	if r.Len() != 3 {
		t.Errorf("Expected router length to be 3, got %d", r.Len())
	}

	_ = r.Delete(nil, "192.168.2.0/24")
	if r.Len() != 2 {
		t.Errorf("Expected router length to be 2 after delete, got %d", r.Len())
	}
}

// 测试Len方法 - Restore后合并路由
func TestRouter_Len_Restore(t *testing.T) {
	r := NewRouter()
	_ = r.Register(nil, "192.168.1.0/24", "target-1")
	_ = r.Register(nil, "192.168.2.0/24", "target-2")

	data := r.Dump()
	initialLen := r.Len()

	// Restore到同一个路由器（合并模式，不覆盖已有路由）
	_ = r.Restore(nil, data)
	if r.Len() != initialLen {
		t.Errorf("Expected router length to remain %d after restore, got %d", initialLen, r.Len())
	}
}

// 测试Len方法 - Restore到新路由器
func TestRouter_Len_RestoreToNewRouter(t *testing.T) {
	r1 := NewRouter()
	for i := 0; i < 100; i++ {
		cidr := fmt.Sprintf("192.168.%d.0/24", i)
		conn := fmt.Sprintf("conn-%d", i)
		_ = r1.Register(nil, cidr, conn)
	}

	data := r1.Dump()
	originalLen := r1.Len()

	// 从数据恢复到新路由器
	r2 := NewRouter(data)
	if r2.Len() != originalLen {
		t.Errorf("Expected new router length to be %d, got %d", originalLen, r2.Len())
	}
}

// 测试Len方法的性能
func BenchmarkRouter_Len(b *testing.B) {
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
				cidr := fmt.Sprintf("192.168.%d.0/24", i%256)
				conn := fmt.Sprintf("conn-%d", i)
				_ = r.Register(nil, cidr, conn)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = r.Len()
			}
		})
	}
}

// 测试RouteTable的Len方法
func TestRouteTable_Len(t *testing.T) {
	table := NewRouteTable()
	if table.Len() != 0 {
		t.Errorf("Expected empty route table length to be 0, got %d", table.Len())
	}

	_ = table.AddRoute(nil, "192.168.1.0/24", "target-1")
	if table.Len() != 1 {
		t.Errorf("Expected route table length to be 1, got %d", table.Len())
	}

	_ = table.AddRoute(nil, "192.168.2.0/24", "target-2")
	_ = table.AddRoute(nil, "192.168.3.0/24", "target-3")
	if table.Len() != 3 {
		t.Errorf("Expected route table length to be 3, got %d", table.Len())
	}
}

// 测试List方法 - 空路由表
func TestRouter_List_Empty(t *testing.T) {
	r := NewRouter()
	result := r.List()
	if result == nil {
		t.Error("Expected non-nil slice for empty router")
	}
	if len(result) != 0 {
		t.Errorf("Expected empty list length to be 0, got %d", len(result))
	}
}

// 测试List方法 - 单条路由
func TestRouter_List_Single(t *testing.T) {
	r := NewRouter()
	_ = r.Register(nil, "192.168.1.0/24", "target-1")

	result := r.List()
	if len(result) != 1 {
		t.Errorf("Expected list length to be 1, got %d", len(result))
	}
	if result[0] != "192.168.1.0/24" {
		t.Errorf("Expected route '192.168.1.0/24', got '%s'", result[0])
	}
}

// 测试List方法 - 多条路由
func TestRouter_List_Multiple(t *testing.T) {
	r := NewRouter()
	expectedRoutes := []string{
		"192.168.1.0/24",
		"192.168.2.0/24",
		"192.168.3.0/24",
		"10.0.1.0/24",
		"10.0.2.0/24",
	}

	for _, route := range expectedRoutes {
		_ = r.Register(nil, route, "target")
	}

	result := r.List()
	if len(result) != len(expectedRoutes) {
		t.Errorf("Expected list length to be %d, got %d", len(expectedRoutes), len(result))
	}

	// 验证所有路由都在结果中
	resultMap := make(map[string]bool)
	for _, route := range result {
		resultMap[route] = true
	}
	for _, expected := range expectedRoutes {
		if !resultMap[expected] {
			t.Errorf("Expected route '%s' not found in result", expected)
		}
	}
}

// 测试List方法 - 不同路由数量
func TestRouter_List_Scales(t *testing.T) {
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
			r := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				octet1 := 10 + i/256
				octet2 := i % 256
				cidr := fmt.Sprintf("%d.%d.0.0/16", octet1, octet2)
				_ = r.Register(nil, cidr, fmt.Sprintf("conn-%d", i))
			}

			result := r.List()
			if len(result) != tc.routeCount {
				t.Errorf("Expected list length to be %d, got %d", tc.routeCount, len(result))
			}
		})
	}
}

// 测试List方法 - 不同前缀长度
func TestRouter_List_DifferentPrefixLengths(t *testing.T) {
	r := NewRouter()
	routes := []string{
		"0.0.0.0/0",
		"10.0.0.0/8",
		"192.168.0.0/16",
		"192.168.1.0/24",
		"192.168.1.1/32",
	}

	for _, route := range routes {
		_ = r.Register(nil, route, "target")
	}

	result := r.List()
	if len(result) != len(routes) {
		t.Errorf("Expected list length to be %d, got %d", len(routes), len(result))
	}

	// 验证所有路由都在结果中
	resultMap := make(map[string]bool)
	for _, route := range result {
		resultMap[route] = true
	}
	for _, expected := range routes {
		if !resultMap[expected] {
			t.Errorf("Expected route '%s' not found in result", expected)
		}
	}
}

// 测试List方法 - AddRoute后更新
func TestRouter_List_AddRoute(t *testing.T) {
	r := NewRouter()
	_ = r.Register(nil, "192.168.1.0/24", "target-1")

	result1 := r.List()
	if len(result1) != 1 {
		t.Errorf("Expected list length to be 1, got %d", len(result1))
	}

	_ = r.Register(nil, "192.168.2.0/24", "target-2")
	result2 := r.List()
	if len(result2) != 2 {
		t.Errorf("Expected list length to be 2, got %d", len(result2))
	}
}

// 测试List方法 - DeleteRoute后更新
func TestRouter_List_DeleteRoute(t *testing.T) {
	r := NewRouter()
	routes := []string{
		"192.168.1.0/24",
		"192.168.2.0/24",
		"192.168.3.0/24",
	}

	for _, route := range routes {
		_ = r.Register(nil, route, "target")
	}

	result1 := r.List()
	if len(result1) != 3 {
		t.Errorf("Expected list length to be 3, got %d", len(result1))
	}

	_ = r.Delete(nil, "192.168.2.0/24")
	result2 := r.List()
	if len(result2) != 2 {
		t.Errorf("Expected list length to be 2 after delete, got %d", len(result2))
	}

	// 验证删除的路由不在结果中
	resultMap := make(map[string]bool)
	for _, route := range result2 {
		resultMap[route] = true
	}
	if resultMap["192.168.2.0/24"] {
		t.Error("Deleted route '192.168.2.0/24' should not be in result")
	}
}

// 测试List方法 - Restore后合并路由
func TestRouter_List_Restore(t *testing.T) {
	r := NewRouter()
	_ = r.Register(nil, "192.168.1.0/24", "target-1")
	_ = r.Register(nil, "192.168.2.0/24", "target-2")

	data := r.Dump()
	initialLen := r.Len()

	// Restore到同一个路由器（合并模式，不覆盖已有路由）
	_ = r.Restore(nil, data)
	result := r.List()
	if len(result) != initialLen {
		t.Errorf("Expected list length to remain %d after restore, got %d", initialLen, len(result))
	}
}

// 测试List方法 - Restore到新路由器
func TestRouter_List_RestoreToNewRouter(t *testing.T) {
	r1 := NewRouter()
	for i := 0; i < 100; i++ {
		cidr := fmt.Sprintf("192.168.%d.0/24", i)
		conn := fmt.Sprintf("conn-%d", i)
		_ = r1.Register(nil, cidr, conn)
	}

	data := r1.Dump()
	originalList := r1.List()

	// 从数据恢复到新路由器（target不序列化，只恢复结构）
	r2 := NewRouter(data)
	result := r2.List()

	// 路由条数应该相同
	if len(result) != len(originalList) {
		t.Errorf("Expected new router list length to be %d, got %d", len(originalList), len(result))
	}

	// 验证所有CIDR都在结果中
	resultMap := make(map[string]bool)
	for _, route := range result {
		resultMap[route] = true
	}
	for _, expected := range originalList {
		if !resultMap[expected] {
			t.Errorf("Expected route '%s' not found in result", expected)
		}
	}
}

// 测试List方法的性能
func BenchmarkRouter_List(b *testing.B) {
	testCases := []struct {
		name       string
		routeCount int
	}{
		{"10 routes", 10},
		{"50 routes", 50},
		{"100 routes", 100},
		{"254 routes", 254},
		{"1000 routes", 1000},
		{"5000 routes", 5000},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			r := NewRouter()
			for i := 0; i < tc.routeCount; i++ {
				cidr := fmt.Sprintf("192.168.%d.0/24", i%256)
				conn := fmt.Sprintf("conn-%d", i)
				_ = r.Register(nil, cidr, conn)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = r.List()
			}
		})
	}
}

// 测试RouteTable的List方法
func TestRouteTable_List(t *testing.T) {
	table := NewRouteTable()
	if len(table.List()) != 0 {
		t.Errorf("Expected empty route table list length to be 0, got %d", len(table.List()))
	}

	_ = table.AddRoute(nil, "192.168.1.0/24", "target-1")
	result := table.List()
	if len(result) != 1 {
		t.Errorf("Expected route table list length to be 1, got %d", len(result))
	}
	if result[0] != "192.168.1.0/24" {
		t.Errorf("Expected route '192.168.1.0/24', got '%s'", result[0])
	}

	_ = table.AddRoute(nil, "192.168.2.0/24", "target-2")
	_ = table.AddRoute(nil, "192.168.3.0/24", "target-3")
	if len(table.List()) != 3 {
		t.Errorf("Expected route table list length to be 3, got %d", len(table.List()))
	}
}

func TestRouter_DeleteAndDump(t *testing.T) {
	r := NewRouter()
	_ = r.Register(nil, "192.168.1.0/24", "target-1")
	_ = r.Register(nil, "192.168.2.0/24", "target-2")
	_ = r.Register(nil, "192.168.3.0/24", "target-3")
	before := r.Dump()
	t.Logf("before: %v bytes", len(base64.StdEncoding.EncodeToString(before)))
	_ = r.Delete(nil, "192.168.2.0/24")
	after := r.Dump()
	t.Logf("after: %v bytes", len(base64.StdEncoding.EncodeToString(after)))
	if len(before) != len(after) {
		t.Errorf("Expected dump length to be %d, got %d", len(before), len(after))
	}
}
