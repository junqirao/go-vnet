package model

import (
	"go-vnet/vnet/server"
)

type (
	GenericInfo struct {
		Server  ServerGenericInfo  `json:"server"`
		P2P     P2PGenericInfo     `json:"p2p"`
		Network NetworkGenericInfo `json:"network"`
	}
	ServerGenericInfo struct {
		Transports []*server.TransportConfig `json:"transports"`
	}
	NetworkGenericInfo struct {
		Running             int    `json:"running"`
		Sessions            int    `json:"sessions"`
		Connections         int    `json:"connections"`
		TotalProxyRxBytes   uint64 `json:"total_proxy_rx_bytes"`
		TotalProxyRxSpeed   uint64 `json:"total_proxy_rx_speed"`
		TotalProxyTxBytes   uint64 `json:"total_proxy_tx_bytes"`
		TotalProxyTxSpeed   uint64 `json:"total_proxy_tx_speed"`
		TotalProxyRxPackets uint64 `json:"total_proxy_rx_packets"`
		TotalProxyTxPackets uint64 `json:"total_proxy_tx_packets"`
	}
	P2PGenericInfo struct {
		Running int `json:"running"`
	}
)
