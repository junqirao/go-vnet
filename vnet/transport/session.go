package transport

import (
	"context"
)

const (
	TypeQuic Type = "quic"
)

type (
	HandshakeHandler interface {
		Handshake() (err error)
	}
	Type    string
	Session struct {
		Ctx            context.Context `json:"-"`
		Conn           any             `json:"-"`
		NetworkId      string          `json:"network_id"`
		SessionId      string          `json:"session_id"`
		Type           Type            `json:"type"`
		ConnectionInfo ConnectionInfo  `json:"connection_info"`
		Meta           map[string]any  `json:"meta"`
	}
	ConnectionInfo struct {
		FromIp    string `json:"from_ip"`
		FromPort  int    `json:"from_port"`
		BatchSize int    `json:"batch_size"`
	}
)

func (t Type) String() string {
	return string(t)
}
