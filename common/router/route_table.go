package router

import (
	"errors"
	"net"
)

// TrieNode 表示前缀树的节点
type TrieNode struct {
	zero   *TrieNode // 0分支
	one    *TrieNode // 1分支
	isLeaf bool      // 是否为叶子节点
	target any       // 路由目标，如"any"
}

// RouteTable 表示IP路由表
type RouteTable struct {
	root *TrieNode // 根节点
}

// NewRouteTable 创建一个新的路由表
func NewRouteTable(root ...*TrieNode) *RouteTable {
	r := &TrieNode{}
	if len(root) > 0 && root[0] != nil {
		r = root[0]
	}
	return &RouteTable{root: r}
}

// AddRoute 添加路由条目，cidr格式如"192.168.1.0/24"，target如"any"
func (rt *RouteTable) AddRoute(cidr string, target any) error {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return errors.New("only IPv4 supported")
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return errors.New("only IPv4 supported")
	}

	current := rt.root
	for i := 0; i < ones; i++ {
		byteIndex := i / 8
		bitOffset := 7 - (i % 8) // 高位在前
		bitVal := (ip4[byteIndex] >> uint(bitOffset)) & 1

		if bitVal == 0 {
			if current.zero == nil {
				current.zero = &TrieNode{}
			}
			current = current.zero
		} else {
			if current.one == nil {
				current.one = &TrieNode{}
			}
			current = current.one
		}
	}
	current.isLeaf = true
	current.target = target
	return nil
}

// DeleteRoute 删除路由条目
func (rt *RouteTable) DeleteRoute(cidr string) error {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return errors.New("only IPv4 supported")
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 32 {
		return errors.New("only IPv4 supported")
	}

	current := rt.root
	for i := 0; i < ones; i++ {
		byteIndex := i / 8
		bitOffset := 7 - (i % 8)
		bitVal := (ip4[byteIndex] >> uint(bitOffset)) & 1

		if bitVal == 0 {
			if current.zero == nil {
				return errors.New("route not found")
			}
			current = current.zero
		} else {
			if current.one == nil {
				return errors.New("route not found")
			}
			current = current.one
		}
	}
	if !current.isLeaf {
		return errors.New("route not found")
	}
	current.isLeaf = false
	current.target = nil
	return nil
}

// Lookup 查找IP对应的路由，返回目标及是否匹配
func (rt *RouteTable) Lookup(ipStr string) (any, bool) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, false
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, false
	}

	current := rt.root
	var bestMatch any
	var found bool
	for i := 0; i < 32; i++ {
		if current.isLeaf {
			bestMatch = current.target
			found = true
		}
		byteIndex := i / 8
		bitOffset := 7 - (i % 8)
		bitVal := (ip4[byteIndex] >> uint(bitOffset)) & 1

		if bitVal == 0 {
			if current.zero == nil {
				break
			}
			current = current.zero
		} else {
			if current.one == nil {
				break
			}
			current = current.one
		}
	}
	if current.isLeaf {
		bestMatch = current.target
		found = true
	}
	if found {
		return bestMatch, true
	}
	return nil, false
}

// MarshalTriNode 序列化TrieNode为字节数组
// 优化格式：
// - bit 0: isLeaf
// - bit 1: hasZero
// - bit 2: hasOne
// - bit 3: zeroLenInFlags (如果为1，说明zero长度编码在flags的bit 4-7)
// - bit 4-7: zeroLen (如果bit 3为1，存储zero长度，范围0-15)
// 然后依次是：zero长度(如果bit 3为0)、zero数据、one长度、one数据
func MarshalTriNode(tr *TrieNode) []byte {
	if tr == nil {
		return []byte{}
	}

	var data []byte

	// 预先序列化子节点以获取长度
	var zeroData, oneData []byte
	if tr.zero != nil {
		zeroData = MarshalTriNode(tr.zero)
	}
	if tr.one != nil {
		oneData = MarshalTriNode(tr.one)
	}

	// 序列化当前节点信息
	var flags byte
	if tr.isLeaf {
		flags |= 1 << 0
	}
	if tr.zero != nil {
		flags |= 1 << 1
	}
	if tr.one != nil {
		flags |= 1 << 2
	}
	data = append(data, flags)

	// 序列化zero子节点
	if tr.zero != nil {
		zeroLen := len(zeroData)
		if zeroLen < 16 {
			// 长度小于16，编码在flags字节中
			flags |= (1 << 3) | (byte(zeroLen) << 4)
			data[0] = flags // 更新第一个字节
		} else {
			// 长度>=16，使用变长编码
			data = append(data, encodeVarUint(uint32(zeroLen))...)
		}
		data = append(data, zeroData...)
	}

	// 序列化one子节点
	if tr.one != nil {
		oneLen := len(oneData)
		data = append(data, encodeVarUint(uint32(oneLen))...)
		data = append(data, oneData...)
	}

	return data
}

// encodeVarUint 编码变长无符号整数，每个字节使用7位存储数据，最高位表示是否还有后续字节
func encodeVarUint(n uint32) []byte {
	if n == 0 {
		return []byte{0}
	}
	var buf []byte
	for n > 0 {
		b := byte(n & 0x7F)
		n >>= 7
		if n > 0 {
			b |= 0x80 // 设置最高位表示还有后续字节
		}
		buf = append(buf, b)
	}
	return buf
}

// decodeVarUint 解码变长无符号整数
func decodeVarUint(bs []byte, offset int) (uint32, int, error) {
	var result uint32
	var shift uint
	for i := offset; i < len(bs); i++ {
		b := bs[i]
		result |= uint32(b&0x7F) << shift
		shift += 7
		if (b & 0x80) == 0 {
			return result, i - offset + 1, nil
		}
		if shift >= 35 {
			return 0, 0, errors.New("varuint too large")
		}
	}
	return 0, 0, errors.New("invalid varuint: unexpected end of data")
}

// UnMarshalTriNode 从字节数组反序列化为TrieNode
func UnMarshalTriNode(bs []byte) (*TrieNode, error) {
	if len(bs) == 0 {
		return nil, nil
	}

	tr := &TrieNode{}
	offset := 0

	// 解析标志位
	flags := bs[offset]
	offset++

	tr.isLeaf = (flags & (1 << 0)) != 0
	hasZero := (flags & (1 << 1)) != 0
	hasOne := (flags & (1 << 2)) != 0

	// 解析zero子节点
	if hasZero {
		var zeroLen uint32
		if (flags & (1 << 3)) != 0 {
			// zero长度编码在flags的bit 4-7
			zeroLen = uint32((flags >> 4) & 0x0F)
		} else {
			// 使用变长解码长度
			var bytesConsumed int
			var err error
			zeroLen, bytesConsumed, err = decodeVarUint(bs, offset)
			if err != nil {
				return nil, err
			}
			offset += bytesConsumed
		}

		if offset+int(zeroLen) > len(bs) {
			return nil, errors.New("invalid data: zero child data out of bounds")
		}
		zeroNode, err := UnMarshalTriNode(bs[offset : offset+int(zeroLen)])
		if err != nil {
			return nil, err
		}
		tr.zero = zeroNode
		offset += int(zeroLen)
	}

	// 解析one子节点
	if hasOne {
		oneLen, bytesConsumed, err := decodeVarUint(bs, offset)
		if err != nil {
			return nil, err
		}
		offset += bytesConsumed

		if offset+int(oneLen) > len(bs) {
			return nil, errors.New("invalid data: one child data out of bounds")
		}
		oneNode, err := UnMarshalTriNode(bs[offset : offset+int(oneLen)])
		if err != nil {
			return nil, err
		}
		tr.one = oneNode
		offset += int(oneLen)
	}

	return tr, nil
}

// MarshalRouteTable 序列化整个路由表
func (rt *RouteTable) MarshalRouteTable() []byte {
	return MarshalTriNode(rt.root)
}
