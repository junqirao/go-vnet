package device

import (
	"runtime"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

type (
	// wireGuardDevice ...
	// needs dll in windows
	wireGuardDevice struct {
		device          tun.Device
		bufferPool      sync.Pool
		sizePool        sync.Pool
		writeVirtioHdr  bool // Linux 上是否启用 VNET 头部
		virtioHdrOffset int  // Linux 上需要的 offset 值
	}
)

// 初始化 writeVirtioHdr 和 virtioHdrOffset
func init() {
	// Linux 平台需要 virtioNetHdrLen，Windows 不需要
	if runtime.GOOS == "linux" {
		// virtioNetHdr 是 golang.zx2c4.com/wireguard 库中定义的结构体
		// 参考：tun/offload_linux.go:868 - if offset < virtioNetHdrLen || offset > len(bufs[i])-1
		// virtioNetHdr 包含：flags(1), gsoType(1), hdrLen(2), gsoSize(2), csumStart(2), csumOffset(2)
		// 总共 10 字节
		wireguardDefaultVirtioHdrOffset = 10
	} else {
		// Windows 平台不需要 virtioNetHdrLen
		wireguardDefaultVirtioHdrOffset = 0
	}
}

// 默认值变量
var (
	wireguardDefaultVirtioHdrOffset int
)

func (w *wireGuardDevice) Name() string {
	name, _ := w.device.Name()
	return name
}

func (w *wireGuardDevice) Close() error {
	return w.device.Close()
}

func (w *wireGuardDevice) Read(packet []byte) (n int, err error) {
	// WireGuard TUN 设备的 Read 函数需要 [][]byte 和 []int 类型的参数
	// 使用临时 buffer 来避免复杂的 pool 逻辑
	var (
		sizes = make([]int, 1)
		buf   = make([][]byte, 1)
	)

	// 重要：确保 packet 有足够的空间来读取数据
	// 在 Linux 上，WireGuard TUN 设备启用了 VNET 头部和 GSO
	// 可能会读取超过用户预期大小的数据
	// 如果 packet 太小，可能会导致 "too many segments" 错误
	minSize := len(packet)
	if wireguardDefaultVirtioHdrOffset > 0 {
		// Linux 平台：确保至少有 65535 字节的空间
		if minSize < 65535 {
			minSize = 65535
		}
	}
	buf[0] = make([]byte, minSize)

	// Read from the device
	_, err = w.device.Read(buf, sizes, 0)
	if err != nil {
		return 0, err
	}

	// 复制读取的数据到 packet
	n = sizes[0]
	if n > len(packet) {
		n = len(packet)
	}
	copy(packet, buf[0][:n])

	return n, nil
}

func (w *wireGuardDevice) Write(packet []byte) (n int, err error) {
	// WireGuard TUN 设备的 Write 函数需要 [][]byte 类型的参数
	// 检查 packet 是否为空
	if len(packet) == 0 {
		return 0, nil
	}

	// 如果启用了 VNET 头部（Linux），需要在数据包前面预留空间
	// 因为 handleGRO 会在 offset-virtioNetHdrLen 位置编码 virtioNetHdr
	// 所以我们需要让 packet 从 offset 开始，而不是从 0 开始
	if wireguardDefaultVirtioHdrOffset > 0 {
		// Linux 平台：创建一个新 buffer，前面预留 virtioNetHdrLen（10）字节
		// 这样 handleGRO 可以在 [offset-virtioNetHdrLen] = [0] 位置编码 virtioNetHdr
		buf := make([]byte, len(packet)+wireguardDefaultVirtioHdrOffset)
		copy(buf[wireguardDefaultVirtioHdrOffset:], packet)
		return w.device.Write([][]byte{buf}, wireguardDefaultVirtioHdrOffset)
	}

	// Windows 平台：直接写入
	return w.device.Write([][]byte{packet}, wireguardDefaultVirtioHdrOffset)
}
