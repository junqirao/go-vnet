package client

type (
	Config struct {
		Server string `json:"server"`
		Type   string `json:"type"`
	}
)

const (
	TypeQuic = "quic"
)
