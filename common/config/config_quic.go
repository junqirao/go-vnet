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
		MaxStreamReceiveWindow:         64 * 1024 * 1024,  // 流接收窗口上限256MB
		InitialStreamReceiveWindow:     16 * 1024 * 1024,  // 初始流接收窗口128MB
		InitialConnectionReceiveWindow: 64 * 1024 * 1024,  // 初始连接接收窗口256MB
		MaxConnectionReceiveWindow:     128 * 1024 * 1024, // 连接接收窗口上限512MB
		Allow0RTT:                      true,
	}
)
