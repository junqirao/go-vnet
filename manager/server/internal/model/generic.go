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
		Total               int    `json:"total"`
		Running             int    `json:"running"`
		Sessions            int    `json:"sessions"`
		Connections         int    `json:"connections"`
		TotalProxyRxSpeed   uint64 `json:"total_proxy_rx_speed"`
		TotalProxyTxSpeed   uint64 `json:"total_proxy_tx_speed"`
		TotalProxyBytes     uint64 `json:"total_proxy_bytes"`
		TotalProxyRxPackets uint64 `json:"total_proxy_rx_packets"`
		TotalProxyTxPackets uint64 `json:"total_proxy_tx_packets"`
	}
	P2PGenericInfo struct {
		Running int `json:"running"`
	}
)
