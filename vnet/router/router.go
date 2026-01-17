package router

import (
	"crypto/md5"
	"encoding/hex"
	"net"
	"sync/atomic"
)

type (
	// Router 高性能无锁路由器，支持 IPv4 和 IPv6
	Router struct {
		v4 *TrieNode // IPv4 路由树
		v6 *TrieNode // IPv6 路由树
	}

	// TrieNode 前缀树节点
	TrieNode struct {
		zero   atomic.Pointer[TrieNode] // 0 分支
		one    atomic.Pointer[TrieNode] // 1 分支
		leaf   atomic.Bool              // 是否为叶子节点
		target atomic.Pointer[any]      // 路由目标
	}
)

func NewRouter() *Router {
	return &Router{
		v4: &TrieNode{},
		v6: &TrieNode{},
	}
}

// Route 通过 IP 包路由，支持 IPv4 和 IPv6
// buf: 二进制 IP 包
// IPv4: 目标地址在 buf[16:20]
// IPv6: 目标地址在 buf[24:40]
func (r *Router) Route(buf []byte) (any, bool) {
	if len(buf) < 20 {
		return nil, false
	}

	// 检查 IP 版本 (IPv4 首字节高4位为 4)
	version := (buf[0] >> 4) & 0x0F

	if version == 4 {
		// IPv4: 检查首部长度字段 (IHL)，计算目标IP偏移
		// IHL 以 4 字节为单位，最小为 5
		if len(buf) < 20 {
			return nil, false
		}
		ihl := buf[0] & 0x0F
		if ihl < 5 || int(ihl*4) > len(buf) {
			return nil, false
		}
		// 目标 IP 地址从偏移 16 开始（对于标准 IPv4 头部）
		if len(buf) < 20 {
			return nil, false
		}
		dstIP := buf[16:20]
		return r.lookupIPv4(dstIP)
	}

	if version == 6 {
		// IPv6: 目标地址从偏移 24 开始
		if len(buf) < 40 {
			return nil, false
		}
		dstIP := buf[24:40]
		return r.lookupIPv6(dstIP)
	}

	return nil, false
}

// RouteString 通过地址字符串路由
// 支持格式: "192.168.1.1", "192.168.1.1/24", "fe80::1", "fe80::1/64"
func (r *Router) RouteString(addr string) (any, bool) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return nil, false
	}

	ip4 := ip.To4()
	if ip4 != nil {
		return r.lookupIPv4(ip4)
	}

	ip6 := ip.To16()
	if ip6 != nil {
		return r.lookupIPv6(ip6)
	}

	return nil, false
}

// Register 注册路由地址
// addr: 路由地址 CIDR 格式，如: "192.168.1.0/24", "fe80::/64"
// val: 路由目标
func (r *Router) Register(addr string, val any) {
	_, ipNet, err := net.ParseCIDR(addr)
	if err != nil {
		return
	}

	ip := ipNet.IP
	ones, _ := ipNet.Mask.Size()

	ip4 := ip.To4()
	if ip4 != nil {
		// IPv4 路由
		r.insertIPv4(ip4, ones, val)
		return
	}

	ip6 := ip.To16()
	if ip6 != nil {
		// IPv6 路由
		r.insertIPv6(ip6, ones, val)
	}
}

// UnRegister 注销路由地址
// addr: 路由地址 CIDR 格式，如: "192.168.1.0/24", "fe80::/64"
func (r *Router) UnRegister(addr string) {
	_, ipNet, err := net.ParseCIDR(addr)
	if err != nil {
		return
	}

	ip := ipNet.IP
	ones, _ := ipNet.Mask.Size()

	ip4 := ip.To4()
	if ip4 != nil {
		r.removeIPv4(ip4, ones)
		return
	}

	ip6 := ip.To16()
	if ip6 != nil {
		r.removeIPv6(ip6, ones)
	}
}

// MD5 返回路由表的 MD5 哈希值
func (r *Router) MD5() string {
	h := md5.New()
	collectMD5(h.(hashWriter), r.v4, "", 0, 128)
	collectMD5(h.(hashWriter), r.v6, "", 0, 128)
	return hex.EncodeToString(h.Sum(nil))
}

// Range 遍历所有路由条目
// 注意: 由于无锁实现，遍历过程中可能有并发修改
func (r *Router) Range(fn func(addr string, val any)) {
	collectRoutes(r.v4, "", 0, 128, fn, true)
	collectRoutes(r.v6, "", 0, 128, fn, false)
}

// Keys 返回所有路由地址列表
func (r *Router) Keys() []string {
	var keys []string
	r.Range(func(addr string, val any) {
		keys = append(keys, addr)
	})
	return keys
}

// ========== IPv4 实现 ==========

// lookupIPv4 在 IPv4 树中查找
func (r *Router) lookupIPv4(ip []byte) (any, bool) {
	if len(ip) != 4 {
		return nil, false
	}

	current := r.v4
	var bestMatch *any
	var found bool

	for i := 0; i < 32; i++ {
		if current.leaf.Load() {
			if target := current.target.Load(); target != nil {
				bestMatch = target
				found = true
			}
		}

		byteIndex := i / 8
		bitOffset := 7 - (i % 8)
		bitVal := (ip[byteIndex] >> uint(bitOffset)) & 1

		if bitVal == 0 {
			next := current.zero.Load()
			if next == nil {
				break
			}
			current = next
		} else {
			next := current.one.Load()
			if next == nil {
				break
			}
			current = next
		}
	}

	// 检查最后一个节点
	if current.leaf.Load() {
		if target := current.target.Load(); target != nil {
			bestMatch = target
			found = true
		}
	}

	if found && bestMatch != nil {
		return *bestMatch, true
	}
	return nil, false
}

// insertIPv4 在 IPv4 树中插入
func (r *Router) insertIPv4(ip []byte, ones int, val any) {
	if len(ip) != 4 {
		return
	}

	current := r.v4
	valuePtr := new(any)
	*valuePtr = val

	for i := 0; i < ones; i++ {
		byteIndex := i / 8
		bitOffset := 7 - (i % 8)
		bitVal := (ip[byteIndex] >> uint(bitOffset)) & 1

		var next *TrieNode
		if bitVal == 0 {
			next = current.zero.Load()
			if next == nil {
				next = &TrieNode{}
				if !current.zero.CompareAndSwap(nil, next) {
					// CAS 失败，重新读取
					next = current.zero.Load()
				}
			}
		} else {
			next = current.one.Load()
			if next == nil {
				next = &TrieNode{}
				if !current.one.CompareAndSwap(nil, next) {
					// CAS 失败，重新读取
					next = current.one.Load()
				}
			}
		}
		current = next
	}

	current.leaf.Store(true)
	current.target.Store(valuePtr)
}

// removeIPv4 在 IPv4 树中删除
func (r *Router) removeIPv4(ip []byte, ones int) {
	if len(ip) != 4 {
		return
	}

	current := r.v4

	for i := 0; i < ones; i++ {
		byteIndex := i / 8
		bitOffset := 7 - (i % 8)
		bitVal := (ip[byteIndex] >> uint(bitOffset)) & 1

		if bitVal == 0 {
			next := current.zero.Load()
			if next == nil {
				return
			}
			current = next
		} else {
			next := current.one.Load()
			if next == nil {
				return
			}
			current = next
		}
	}

	current.leaf.Store(false)
	current.target.Store(nil)
}

// ========== IPv6 实现 ==========

// lookupIPv6 在 IPv6 树中查找
func (r *Router) lookupIPv6(ip []byte) (any, bool) {
	if len(ip) != 16 {
		return nil, false
	}

	current := r.v6
	var bestMatch *any
	var found bool

	for i := 0; i < 128; i++ {
		if current.leaf.Load() {
			if target := current.target.Load(); target != nil {
				bestMatch = target
				found = true
			}
		}

		byteIndex := i / 8
		bitOffset := 7 - (i % 8)
		bitVal := (ip[byteIndex] >> uint(bitOffset)) & 1

		if bitVal == 0 {
			next := current.zero.Load()
			if next == nil {
				break
			}
			current = next
		} else {
			next := current.one.Load()
			if next == nil {
				break
			}
			current = next
		}
	}

	if current.leaf.Load() {
		if target := current.target.Load(); target != nil {
			bestMatch = target
			found = true
		}
	}

	if found && bestMatch != nil {
		return *bestMatch, true
	}
	return nil, false
}

// insertIPv6 在 IPv6 树中插入
func (r *Router) insertIPv6(ip []byte, ones int, val any) {
	if len(ip) != 16 {
		return
	}

	current := r.v6
	valuePtr := new(any)
	*valuePtr = val

	for i := 0; i < ones; i++ {
		byteIndex := i / 8
		bitOffset := 7 - (i % 8)
		bitVal := (ip[byteIndex] >> uint(bitOffset)) & 1

		var next *TrieNode
		if bitVal == 0 {
			next = current.zero.Load()
			if next == nil {
				next = &TrieNode{}
				if !current.zero.CompareAndSwap(nil, next) {
					next = current.zero.Load()
				}
			}
		} else {
			next = current.one.Load()
			if next == nil {
				next = &TrieNode{}
				if !current.one.CompareAndSwap(nil, next) {
					next = current.one.Load()
				}
			}
		}
		current = next
	}

	current.leaf.Store(true)
	current.target.Store(valuePtr)
}

// removeIPv6 在 IPv6 树中删除
func (r *Router) removeIPv6(ip []byte, ones int) {
	if len(ip) != 16 {
		return
	}

	current := r.v6

	for i := 0; i < ones; i++ {
		byteIndex := i / 8
		bitOffset := 7 - (i % 8)
		bitVal := (ip[byteIndex] >> uint(bitOffset)) & 1

		if bitVal == 0 {
			next := current.zero.Load()
			if next == nil {
				return
			}
			current = next
		} else {
			next := current.one.Load()
			if next == nil {
				return
			}
			current = next
		}
	}

	current.leaf.Store(false)
	current.target.Store(nil)
}

// ========== 辅助函数 ==========

// collectMD5 递归收集路由信息计算 MD5
func collectMD5(h hashWriter, node *TrieNode, prefix string, depth, maxDepth int) {
	if node == nil || depth >= maxDepth {
		return
	}

	if node.leaf.Load() && node.target.Load() != nil {
		h.Write([]byte(prefix))
	}

	zero := node.zero.Load()
	if zero != nil {
		collectMD5(h, zero, prefix+"0", depth+1, maxDepth)
	}

	one := node.one.Load()
	if one != nil {
		collectMD5(h, one, prefix+"1", depth+1, maxDepth)
	}
}

// hashWriter 定义 hash 接口
type hashWriter interface {
	Write([]byte) (int, error)
}

// collectRoutes 递归收集所有路由
func collectRoutes(node *TrieNode, prefix string, depth, maxDepth int, fn func(addr string, val any), isIPv4 bool) {
	if node == nil || depth >= maxDepth {
		return
	}

	if node.leaf.Load() {
		if target := node.target.Load(); target != nil {
			cidr := prefixToCIDR(prefix, depth, isIPv4)
			fn(cidr, *target)
		}
	}

	zero := node.zero.Load()
	if zero != nil {
		collectRoutes(zero, prefix+"0", depth+1, maxDepth, fn, isIPv4)
	}

	one := node.one.Load()
	if one != nil {
		collectRoutes(one, prefix+"1", depth+1, maxDepth, fn, isIPv4)
	}
}

// prefixToCIDR 将二进制前缀转换为 CIDR 格式
func prefixToCIDR(prefix string, bits int, isIPv4 bool) string {
	if bits == 0 {
		if isIPv4 {
			return "0.0.0.0/0"
		}
		return "::/0"
	}

	if isIPv4 {
		ip := make([]byte, 4)
		for i := 0; i < 32 && i < len(prefix); i++ {
			if prefix[i] == '1' {
				byteIndex := i / 8
				bitOffset := 7 - (i % 8)
				ip[byteIndex] |= 1 << bitOffset
			}
		}
		// 清除网络位之后的 Host 位
		if bits < 32 {
			lastByte := bits / 8
			lastBits := bits % 8
			for i := lastByte + 1; i < 4; i++ {
				ip[i] = 0
			}
			if lastBits > 0 {
				ip[lastByte] &= ^byte(0xFF >> lastBits)
			}
		}
		return ip4ToString(ip) + "/" + intToString(bits)
	}

	// IPv6
	ip := make([]byte, 16)
	for i := 0; i < 128 && i < len(prefix); i++ {
		if prefix[i] == '1' {
			byteIndex := i / 8
			bitOffset := 7 - (i % 8)
			ip[byteIndex] |= 1 << bitOffset
		}
	}
	// 清除网络位之后的 Host 位
	if bits < 128 {
		lastByte := bits / 8
		lastBits := bits % 8
		for i := lastByte + 1; i < 16; i++ {
			ip[i] = 0
		}
		if lastBits > 0 {
			ip[lastByte] &= ^byte(0xFF >> lastBits)
		}
	}
	return ip6ToString(ip) + "/" + intToString(bits)
}

// ip4ToString 将 []byte 转换为 IPv4 字符串
func ip4ToString(ip []byte) string {
	return intToString(int(ip[0])) + "." +
		intToString(int(ip[1])) + "." +
		intToString(int(ip[2])) + "." +
		intToString(int(ip[3]))
}

// ip6ToString 将 []byte 转换为 IPv6 字符串
func ip6ToString(ip []byte) string {
	result := ""
	for i := 0; i < 16; i += 2 {
		if i > 0 {
			result += ":"
		}
		val := uint16(ip[i])<<8 | uint16(ip[i+1])
		result += uint16ToHex(val)
	}
	return result
}

// uint16ToHex 将 uint16 转换为十六进制字符串
func uint16ToHex(v uint16) string {
	const hexDigits = "0123456789abcdef"
	if v == 0 {
		return "0"
	}
	result := ""
	for v > 0 {
		result = string(hexDigits[v&0xF]) + result
		v >>= 4
	}
	return result
}

// intToString 将 int 转换为字符串
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	result := ""
	for n > 0 {
		result = string(digits[n%10]) + result
		n /= 10
	}
	return result
}
