package config

import (
	"time"

	"github.com/quic-go/quic-go"
)

var (
	DefaultQuicConfig = &quic.Config{
		KeepAlivePeriod:                time.Second * 3,
		EnableDatagrams:                true,
		MaxIdleTimeout:                 time.Second * 30,
		MaxIncomingStreams:             1000,
		MaxIncomingUniStreams:          1000,
		MaxStreamReceiveWindow:         8 * 1024 * 1024,  // 优化：增大流接收窗口到8MB，提高吞吐量
		InitialConnectionReceiveWindow: 16 * 1024 * 1024, // 优化：增大初始连接接收窗口到16MB，加快冷启动
		MaxConnectionReceiveWindow:     64 * 1024 * 1024, // 优化：设置连接接收窗口上限64MB
		Allow0RTT:                      true,
	}
)
