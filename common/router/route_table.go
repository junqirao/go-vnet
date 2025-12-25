package router

import (
	"encoding/binary"
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
func MarshalTriNode(tr *TrieNode) []byte {
	if tr == nil {
		return []byte{}
	}

	var data []byte

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

	// 递归序列化子节点
	if tr.zero != nil {
		zeroData := MarshalTriNode(tr.zero)
		zeroLen := make([]byte, 4)
		binary.BigEndian.PutUint32(zeroLen, uint32(len(zeroData)))
		data = append(data, zeroLen...)
		data = append(data, zeroData...)
	}

	if tr.one != nil {
		oneData := MarshalTriNode(tr.one)
		oneLen := make([]byte, 4)
		binary.BigEndian.PutUint32(oneLen, uint32(len(oneData)))
		data = append(data, oneLen...)
		data = append(data, oneData...)
	}

	return data
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

	// 递归解析子节点
	if hasZero {
		if offset+4 > len(bs) {
			return nil, errors.New("invalid data: zero child length out of bounds")
		}
		zeroLen := binary.BigEndian.Uint32(bs[offset : offset+4])
		offset += 4

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

	if hasOne {
		if offset+4 > len(bs) {
			return nil, errors.New("invalid data: one child length out of bounds")
		}
		oneLen := binary.BigEndian.Uint32(bs[offset : offset+4])
		offset += 4

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
