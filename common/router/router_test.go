package router

import (
	"sync"
	"testing"
)

// ========== NewRouter 测试 ==========

func TestNewRouter(t *testing.T) {
	r := NewRouter()
	if r == nil {
		t.Fatal("NewRouter returned nil")
	}
	if r.v4 == nil {
		t.Fatal("IPv4 trie not initialized")
	}
	if r.v6 == nil {
		t.Fatal("IPv6 trie not initialized")
	}
}

// ========== Register 测试 ==========

func TestRegisterIPv4(t *testing.T) {
	r := NewRouter()

	// 测试各种前缀长度
	testCases := []string{
		"0.0.0.0/0",
		"192.168.0.0/16",
		"192.168.1.0/24",
		"192.168.1.100/32",
	}

	for i, cidr := range testCases {
		r.Register(cidr, i)
	}

	// 验证路由已注册
	val, ok := r.RouteString("192.168.1.100")
	if !ok || val != 3 {
		t.Errorf("Expected 3, got %v, ok=%v", val, ok)
	}
}

func TestRegisterIPv6(t *testing.T) {
	r := NewRouter()

	testCases := []string{
		"::/0",
		"fe80::/10",
		"fe80::1/64",
		"2001:db8::1/128",
	}

	for i, cidr := range testCases {
		r.Register(cidr, i)
	}

	// 验证路由已注册
	val, ok := r.RouteString("fe80::1")
	if !ok || val != 2 {
		t.Errorf("Expected 2, got %v, ok=%v", val, ok)
	}
}

func TestRegisterInvalidCIDR(t *testing.T) {
	r := NewRouter()

	// 无效的 CIDR 不应该 panic
	r.Register("invalid", "value")
	r.Register("999.999.999.999/24", "value")

	// 路由表应该仍然有效
	val, ok := r.RouteString("192.168.1.1")
	if ok {
		t.Errorf("Expected no match, got %v", val)
	}
}

// ========== RouteString 测试 ==========

func TestRouteStringIPv4(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.0.0/16", "16bit")
	r.Register("192.168.1.0/24", "24bit")
	r.Register("192.168.1.100/32", "32bit")

	testCases := []struct {
		ip       string
		expected interface{}
		ok       bool
	}{
		{"192.168.1.100", "32bit", true},
		{"192.168.1.1", "24bit", true},
		{"192.168.2.1", "16bit", true},
		{"10.0.0.1", nil, false},
	}

	for _, tc := range testCases {
		val, ok := r.RouteString(tc.ip)
		if ok != tc.ok {
			t.Errorf("RouteString(%s): expected ok=%v, got ok=%v", tc.ip, tc.ok, ok)
		}
		if ok && val != tc.expected {
			t.Errorf("RouteString(%s): expected %v, got %v", tc.ip, tc.expected, val)
		}
	}
}

func TestRouteStringIPv6(t *testing.T) {
	r := NewRouter()
	r.Register("2001:db8::/32", "32bit")
	r.Register("2001:db8:1234::/48", "48bit")
	r.Register("2001:db8:1234:5678::/64", "64bit")

	testCases := []struct {
		ip       string
		expected interface{}
		ok       bool
	}{
		{"2001:db8:1234:5678::1", "64bit", true},
		{"2001:db8:1234:abcd::1", "48bit", true},
		{"2001:db8:abcd::1", "32bit", true},
		{"fe80::1", nil, false},
	}

	for _, tc := range testCases {
		val, ok := r.RouteString(tc.ip)
		if ok != tc.ok {
			t.Errorf("RouteString(%s): expected ok=%v, got ok=%v", tc.ip, tc.ok, ok)
		}
		if ok && val != tc.expected {
			t.Errorf("RouteString(%s): expected %v, got %v", tc.ip, tc.expected, val)
		}
	}
}

func TestRouteStringInvalidIP(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "value")

	val, ok := r.RouteString("invalid")
	if ok || val != nil {
		t.Errorf("Expected (nil, false) for invalid IP, got (%v, %v)", val, ok)
	}
}

// ========== Route 测试 ==========

func TestRouteIPv4Packet(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target")

	// 构造 IPv4 数据包
	packet := make([]byte, 20)
	packet[0] = 0x45 // 版本 4, IHL 5
	packet[16] = 192
	packet[17] = 168
	packet[18] = 1
	packet[19] = 100

	val, ok := r.Route(packet)
	if !ok || val != "target" {
		t.Errorf("Expected target, got %v, ok=%v", val, ok)
	}
}

func TestRouteIPv6Packet(t *testing.T) {
	r := NewRouter()
	r.Register("2001:db8::/32", "target")

	// 构造 IPv6 数据包
	packet := make([]byte, 40)
	packet[0] = 0x60 // 版本 6
	// 目标地址: 2001:0db8:0000:0000:0000:0000:0000:0001
	packet[24] = 0x20
	packet[25] = 0x01
	packet[26] = 0x0d
	packet[27] = 0xb8
	packet[39] = 0x01

	val, ok := r.Route(packet)
	if !ok || val != "target" {
		t.Errorf("Expected target, got %v, ok=%v", val, ok)
	}
}

func TestRouteInvalidPacket(t *testing.T) {
	r := NewRouter()

	// 空包
	val, ok := r.Route([]byte{})
	if ok || val != nil {
		t.Errorf("Expected (nil, false) for empty packet")
	}

	// 短包
	val, ok = r.Route(make([]byte, 10))
	if ok || val != nil {
		t.Errorf("Expected (nil, false) for short packet")
	}

	// 无效版本
	packet := make([]byte, 20)
	packet[0] = 0xFF // 无效版本
	val, ok = r.Route(packet)
	if ok || val != nil {
		t.Errorf("Expected (nil, false) for invalid version")
	}
}

// ========== UnRegister 测试 ==========

func TestUnRegisterIPv4(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target")

	// 验证路由存在
	val, ok := r.RouteString("192.168.1.100")
	if !ok || val != "target" {
		t.Errorf("Route failed before unregister")
	}

	// 注销路由
	r.UnRegister("192.168.1.0/24")

	// 验证路由不存在
	val, ok = r.RouteString("192.168.1.100")
	if ok || val != nil {
		t.Errorf("Expected no match after unregister, got %v", val)
	}
}

func TestUnRegisterIPv6(t *testing.T) {
	r := NewRouter()
	r.Register("2001:db8::/32", "target")

	// 验证路由存在
	val, ok := r.RouteString("2001:db8::1")
	if !ok || val != "target" {
		t.Errorf("Route failed before unregister")
	}

	// 注销路由
	r.UnRegister("2001:db8::/32")

	// 验证路由不存在
	val, ok = r.RouteString("2001:db8::1")
	if ok || val != nil {
		t.Errorf("Expected no match after unregister, got %v", val)
	}
}

func TestUnRegisterNonExistent(t *testing.T) {
	r := NewRouter()

	// 注销不存在的路由不应该 panic
	r.UnRegister("192.168.1.0/24")
	r.UnRegister("2001:db8::/32")
}

func TestUnRegisterSpecificPrefix(t *testing.T) {
	r := NewRouter()

	// 注册多条路由
	r.Register("192.168.0.0/16", "16bit")
	r.Register("192.168.1.0/24", "24bit")
	r.Register("192.168.1.100/32", "32bit")

	// 注销 /24 路由
	r.UnRegister("192.168.1.0/24")

	// 应该匹配 /16
	val, ok := r.RouteString("192.168.1.50")
	if !ok || val != "16bit" {
		t.Errorf("Expected 16bit, got %v, ok=%v", val, ok)
	}

	// 应该匹配 /32
	val, ok = r.RouteString("192.168.1.100")
	if !ok || val != "32bit" {
		t.Errorf("Expected 32bit, got %v, ok=%v", val, ok)
	}
}

// ========== MD5 测试 ==========

func TestMD5(t *testing.T) {
	r := NewRouter()

	// 空路由表
	hash1 := r.MD5()
	if len(hash1) != 32 {
		t.Errorf("Expected MD5 hash length 32, got %d", len(hash1))
	}

	// 添加路由
	r.Register("192.168.1.0/24", "target1")
	hash2 := r.MD5()

	// Hash 应该改变
	if hash1 == hash2 {
		t.Error("MD5 hash should change after adding route")
	}

	// 相同的路由表应该有相同的 Hash
	r2 := NewRouter()
	r2.Register("192.168.1.0/24", "target1")
	hash3 := r2.MD5()

	if hash2 != hash3 {
		t.Errorf("Same routes should have same hash: %s vs %s", hash2, hash3)
	}

	// 添加更多路由
	r.Register("10.0.0.0/8", "target2")
	hash4 := r.MD5()

	if hash3 == hash4 {
		t.Error("MD5 hash should change after adding more routes")
	}
}

func TestMD5AfterUnRegister(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target1")
	r.Register("10.0.0.0/8", "target2")

	hash1 := r.MD5()

	r.UnRegister("192.168.1.0/24")
	hash2 := r.MD5()

	if hash1 == hash2 {
		t.Error("MD5 hash should change after unregister")
	}
}

// ========== Range 测试 ==========

func TestRange(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target1")
	r.Register("10.0.0.0/8", "target2")
	r.Register("172.16.0.0/16", "target3")

	routes := make(map[string]any)
	r.Range(func(addr string, val any) {
		routes[addr] = val
	})

	if len(routes) != 3 {
		t.Errorf("Expected 3 routes, got %d", len(routes))
	}

	expected := map[string]any{
		"192.168.1.0/24": "target1",
		"10.0.0.0/8":     "target2",
		"172.16.0.0/16":  "target3",
	}

	for addr, val := range expected {
		if routes[addr] != val {
			t.Errorf("Route %s: expected %v, got %v", addr, val, routes[addr])
		}
	}
}

func TestRangeEmpty(t *testing.T) {
	r := NewRouter()
	count := 0
	r.Range(func(addr string, val any) {
		count++
	})
	if count != 0 {
		t.Errorf("Expected 0 routes, got %d", count)
	}
}

// ========== Keys 测试 ==========

func TestKeys(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target1")
	r.Register("10.0.0.0/8", "target2")
	r.Register("172.16.0.0/16", "target3")

	keys := r.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// 检查所有键是否存在
	expected := map[string]bool{
		"192.168.1.0/24": true,
		"10.0.0.0/8":     true,
		"172.16.0.0/16":  true,
	}

	for _, key := range keys {
		if !expected[key] {
			t.Errorf("Unexpected key: %s", key)
		}
		delete(expected, key)
	}

	if len(expected) > 0 {
		t.Errorf("Missing keys: %v", expected)
	}
}

func TestKeysEmpty(t *testing.T) {
	r := NewRouter()
	keys := r.Keys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 keys, got %d", len(keys))
	}
}

// ========== 并发测试 ==========

func TestConcurrentRegister(t *testing.T) {
	r := NewRouter()
	var wg sync.WaitGroup

	// 并发注册路由
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cidr := "192.168." + intToString(idx%256) + ".0/24"
			r.Register(cidr, idx)
		}(i)
	}

	wg.Wait()

	// 验证路由已注册
	keys := r.Keys()
	if len(keys) != 100 {
		t.Errorf("Expected 100 routes, got %d", len(keys))
	}
}

func TestConcurrentRoute(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target")

	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.RouteString("192.168.1.100")
		}()
	}

	wg.Wait()
	// 不应该 panic
}

func TestConcurrentRegisterAndRoute(t *testing.T) {
	r := NewRouter()
	var wg sync.WaitGroup

	// 并发注册
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cidr := "192.168." + intToString(idx%256) + ".0/24"
			r.Register(cidr, idx)
		}(i)
	}

	// 并发路由查询
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.RouteString("192.168.1.100")
		}()
	}

	wg.Wait()
	// 不应该 panic
}

func TestConcurrentUnRegister(t *testing.T) {
	r := NewRouter()

	// 先注册一些路由
	for i := 0; i < 100; i++ {
		cidr := "192.168." + intToString(i) + ".0/24"
		r.Register(cidr, i)
	}

	var wg sync.WaitGroup

	// 并发注销
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cidr := "192.168." + intToString(idx) + ".0/24"
			r.UnRegister(cidr)
		}(i)
	}

	wg.Wait()

	keys := r.Keys()
	if len(keys) != 0 {
		t.Errorf("Expected 0 routes after unregister, got %d", len(keys))
	}
}

// ========== 前缀匹配测试 ==========

func TestLongestPrefixMatch(t *testing.T) {
	r := NewRouter()

	// 注册不同长度的前缀
	r.Register("0.0.0.0/0", "default")
	r.Register("10.0.0.0/8", "classA")
	r.Register("10.1.0.0/16", "subnet16")
	r.Register("10.1.2.0/24", "subnet24")
	r.Register("10.1.2.3/32", "host")

	testCases := []struct {
		ip       string
		expected string
	}{
		{"10.1.2.3", "host"},
		{"10.1.2.100", "subnet24"},
		{"10.1.100.1", "subnet16"},
		{"10.100.1.1", "classA"},
		{"172.16.1.1", "default"},
	}

	for _, tc := range testCases {
		val, ok := r.RouteString(tc.ip)
		if !ok || val != tc.expected {
			t.Errorf("RouteString(%s): expected %v, got %v", tc.ip, tc.expected, val)
		}
	}
}

// ========== 大量路由测试 ==========

func TestLargeRouteTable(t *testing.T) {
	r := NewRouter()
	count := 256

	// 注册 256 个 /32 路由
	for i := 0; i < count; i++ {
		cidr := "192.168.0." + intToString(i) + "/32"
		r.Register(cidr, i)
	}

	keys := r.Keys()
	if len(keys) != count {
		t.Errorf("Expected %d routes, got %d", count, len(keys))
	}

	// 测试路由查找
	for i := 0; i < 256; i++ {
		ip := "192.168.0." + intToString(i)
		val, ok := r.RouteString(ip)
		if !ok {
			t.Errorf("RouteString(%s) failed", ip)
		}
		if val != i {
			t.Errorf("RouteString(%s): expected %v, got %v", ip, i, val)
		}
	}

	// 测试不存在的路由
	val, ok := r.RouteString("192.168.1.100")
	if ok {
		t.Errorf("Expected no match for 192.168.1.100, got %v", val)
	}
}

// ========== Benchmark ==========

func BenchmarkRouteStringIPv4(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", "target")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.RouteString("192.168.1.100")
	}
}

func BenchmarkRouteStringIPv6(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("2001:db8::/32", "target")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.RouteString("2001:db8::1")
	}
}

func BenchmarkRouteIPv4(b *testing.B) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target")

	packet := make([]byte, 20)
	packet[0] = 0x45
	packet[16] = 192
	packet[17] = 168
	packet[18] = 1
	packet[19] = 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Route(packet)
	}
}

func BenchmarkRouteIPv6(b *testing.B) {
	r := NewRouter()
	r.Register("2001:db8::/32", "target")

	packet := make([]byte, 40)
	packet[0] = 0x60
	packet[24] = 0x20
	packet[25] = 0x01
	packet[26] = 0x0d
	packet[27] = 0xb8

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Route(packet)
	}
}

func BenchmarkRegisterIPv4(b *testing.B) {
	r := NewRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cidr := "192.168." + intToString(i%256) + "." + intToString((i/256)%256) + "/24"
		r.Register(cidr, "target")
	}
}

func BenchmarkRegisterIPv6(b *testing.B) {
	r := NewRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Register("2001:db8:"+uint16ToHex(uint16(i))+"::/64", "target")
	}
}

func BenchmarkUnRegister(b *testing.B) {
	b.StopTimer()
	r := NewRouter()
	routes := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		cidr := "192.168." + intToString(i%256) + "." + intToString((i/256)%256) + "/24"
		routes[i] = cidr
		r.Register(cidr, "target")
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		r.UnRegister(routes[i%10000])
	}
}

func BenchmarkMD5(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", "target")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.MD5()
	}
}

func BenchmarkKeys(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", "target")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Keys()
	}
}

func BenchmarkRange(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", "target")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		r.Range(func(addr string, val any) {
			count++
		})
	}
}

// ========== 压力测试 ==========

func BenchmarkConcurrentReadWrite(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", i)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.RouteString("192.168.1.100")
		}
	})
}

func BenchmarkConcurrentRegister(b *testing.B) {
	r := NewRouter()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cidr := "192.168." + intToString(i%256) + "." + intToString((i/256)%256) + "/24"
			r.Register(cidr, "target")
			i++
		}
	})
}

// ========== MD5WithValue 测试 ==========

func TestMD5WithValue(t *testing.T) {
	r := NewRouter()

	// 空路由表
	hash1 := r.MD5WithValue()
	if len(hash1) != 32 {
		t.Errorf("Expected MD5 hash length 32, got %d", len(hash1))
	}

	// 添加路由
	r.Register("192.168.1.0/24", "target1")
	hash2 := r.MD5WithValue()

	// Hash 应该改变
	if hash1 == hash2 {
		t.Error("MD5WithValue hash should change after adding route")
	}

	// 相同的路由表和value应该有相同的hash
	r2 := NewRouter()
	r2.Register("192.168.1.0/24", "target1")
	hash3 := r2.MD5WithValue()

	if hash2 != hash3 {
		t.Errorf("Same routes and values should have same hash: %s vs %s", hash2, hash3)
	}

	// 添加更多路由
	r.Register("10.0.0.0/8", "target2")
	hash4 := r.MD5WithValue()

	if hash3 == hash4 {
		t.Error("MD5WithValue hash should change after adding more routes")
	}
}

func TestMD5WithValueDifferentValues(t *testing.T) {
	r1 := NewRouter()
	r1.Register("192.168.1.0/24", "value1")

	r2 := NewRouter()
	r2.Register("192.168.1.0/24", "value2")

	hash1 := r1.MD5WithValue()
	hash2 := r2.MD5WithValue()

	// 不同的值应该产生不同的hash
	if hash1 == hash2 {
		t.Error("MD5WithValue should differ for different values")
	}
}

func TestMD5WithValuevsMD5(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target1")
	r.Register("10.0.0.0/8", "target2")

	hashWithoutValue := r.MD5()
	hashWithValue := r.MD5WithValue()

	// 包含value的hash应该不同
	if hashWithoutValue == hashWithValue {
		t.Error("MD5 and MD5WithValue should produce different hashes")
	}
}

func TestMD5WithValueAfterUnRegister(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target1")
	r.Register("10.0.0.0/8", "target2")

	hash1 := r.MD5WithValue()

	r.UnRegister("192.168.1.0/24")
	hash2 := r.MD5WithValue()

	if hash1 == hash2 {
		t.Error("MD5WithValue hash should change after unregister")
	}
}

func TestMD5WithValueComplexTypes(t *testing.T) {
	r := NewRouter()

	// 测试不同类型的值
	r.Register("192.168.1.0/24", 123)
	r.Register("10.0.0.0/8", "string")
	r.Register("172.16.0.0/16", struct{ Name string }{"test"})

	hash1 := r.MD5WithValue()
	if len(hash1) != 32 {
		t.Errorf("Expected MD5 hash length 32, got %d", len(hash1))
	}

	// 修改值后hash应该改变
	r.UnRegister("192.168.1.0/24")
	r.Register("192.168.1.0/24", 456)

	hash2 := r.MD5WithValue()
	if hash1 == hash2 {
		t.Error("MD5WithValue should change when value changes")
	}
}

func TestMD5WithValueIPv6(t *testing.T) {
	r := NewRouter()

	// IPv6 路由
	r.Register("2001:db8::/32", "ipv6-target")
	r.Register("fe80::/10", "link-local")

	hash1 := r.MD5WithValue()
	if len(hash1) != 32 {
		t.Errorf("Expected MD5 hash length 32, got %d", len(hash1))
	}

	// 相同配置的router应该有相同hash
	r2 := NewRouter()
	r2.Register("2001:db8::/32", "ipv6-target")
	r2.Register("fe80::/10", "link-local")

	hash2 := r2.MD5WithValue()

	if hash1 != hash2 {
		t.Errorf("Same IPv6 routes should have same hash: %s vs %s", hash1, hash2)
	}
}

func TestMD5WithValueCache(t *testing.T) {
	r := NewRouter()
	r.Register("192.168.1.0/24", "target1")

	// 第一次计算
	hash1 := r.MD5WithValue()
	hash2 := r.MD5WithValue()

	// 应该从缓存读取，结果相同
	if hash1 != hash2 {
		t.Error("Cached MD5WithValue should return same hash")
	}

	// 修改路由表
	r.Register("10.0.0.0/8", "target2")
	hash3 := r.MD5WithValue()

	// hash应该改变
	if hash1 == hash3 {
		t.Error("MD5WithValue should change after route modification")
	}
}

func TestMD5WithValueMixedIPVersions(t *testing.T) {
	r := NewRouter()

	// 混合IPv4和IPv6路由
	r.Register("192.168.1.0/24", "v4-target")
	r.Register("2001:db8::/32", "v6-target")
	r.Register("10.0.0.0/8", "v4-target2")
	r.Register("fe80::/10", "v6-target2")

	hash1 := r.MD5WithValue()

	// 相同配置的router应该有相同hash
	r2 := NewRouter()
	r2.Register("192.168.1.0/24", "v4-target")
	r2.Register("2001:db8::/32", "v6-target")
	r2.Register("10.0.0.0/8", "v4-target2")
	r2.Register("fe80::/10", "v6-target2")

	hash2 := r2.MD5WithValue()

	if hash1 != hash2 {
		t.Errorf("Same mixed routes should have same hash: %s vs %s", hash1, hash2)
	}
}

// ========== MD5WithValue Benchmark ==========

func BenchmarkMD5WithValue(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.MD5WithValue()
	}
}

func BenchmarkMD5WithValueCache(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", i)
	}

	// 预先计算一次，填充缓存
	r.MD5WithValue()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.MD5WithValue()
	}
}

func BenchmarkMD5WithValueLargeRoutes(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 10000; i++ {
		r.Register("192.168."+intToString(i%256)+"."+intToString((i/256)%256)+"/32", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.MD5WithValue()
	}
}

func BenchmarkMD5WithValueIPv6(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("2001:db8:"+uint16ToHex(uint16(i))+"::/64", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.MD5WithValue()
	}
}

func BenchmarkMD5WithValueMixedIPVersions(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 500; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", i)
	}
	for i := 0; i < 500; i++ {
		r.Register("2001:db8:"+uint16ToHex(uint16(i))+"::/64", i+500)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.MD5WithValue()
	}
}

func BenchmarkMD5VsMD5WithValue(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", i)
	}

	b.Run("MD5", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r.MD5()
		}
	})

	b.Run("MD5WithValue", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r.MD5WithValue()
		}
	})
}

func BenchmarkMD5WithValueConcurrent(b *testing.B) {
	r := NewRouter()
	for i := 0; i < 1000; i++ {
		r.Register("192.168."+intToString(i%256)+".0/24", i)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			r.MD5WithValue()
		}
	})
}

// ========== MD5WithValue 压力测试 ==========

func BenchmarkMD5WithValueStress(b *testing.B) {
	r := NewRouter()
	// 注册大量路由，模拟真实场景
	for i := 0; i < 65536; i++ {
		cidr := "10." + intToString(i/256) + "." + intToString(i%256) + ".0/24"
		r.Register(cidr, "target-"+intToString(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.MD5WithValue()
	}
}

func BenchmarkMD5WithValueDynamicRoutes(b *testing.B) {
	r := NewRouter()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// 动态注册和计算
			cidr := "192.168." + intToString(i%256) + "." + intToString((i/256)%256) + "/24"
			r.Register(cidr, i)
			r.MD5WithValue()
			if i%100 == 0 {
				// 偶尔删除一些路由
				r.UnRegister("192.168." + intToString((i-100)%256) + ".0/24")
			}
			i++
		}
	})
}

func BenchmarkMD5WithValueDifferentValueTypes(b *testing.B) {
	r := NewRouter()

	// 不同类型的值
	for i := 0; i < 1000; i++ {
		cidr := "192.168." + intToString(i%256) + "." + intToString((i/256)%256) + "/24"
		switch i % 3 {
		case 0:
			r.Register(cidr, i) // int
		case 1:
			r.Register(cidr, intToString(i)) // string
		case 2:
			r.Register(cidr, struct{ Val int }{i}) // struct
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.MD5WithValue()
	}
}
