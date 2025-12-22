package transport

import (
	"context"
	"crypto/tls"
	"strconv"
	"strings"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	"go-vnet/common/logger"
)

var (
	defaultClientConfig = func() *Config {
		return &Config{
			MappedConfig: config.NewMappedConfig(),
			Type:         TypeQuic,
			MTU:          1400,
		}
	}
)

const (
	TypeQuic Type = "quic"
)

type (
	Type   string
	Config struct {
		config.MappedConfig
		authPayload map[string]any

		Port               int    `json:"port"`
		Address            string `json:"address"`
		Type               Type   `json:"type"`
		MTU                int    `json:"mtu"`
		InsecureSkipVerify bool   `json:"insecure_skip_verify"`
	}
	ConfigOption func(cfg *Config)
)

func NewConfig(opts ...ConfigOption) *Config {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// -------------------- OPTIONS --------------------

const (
	configKeyTLS    = "tls"
	configKeyLogger = "logger"
)

func WithTLSConfig(t *tls.Config) ConfigOption {
	return func(cfg *Config) {
		cfg.Set(configKeyTLS, t)
	}
}

func WithAddress(s string) ConfigOption {
	return func(cfg *Config) {
		part := strings.Split(s, ":")
		if len(part) > 1 {
			cfg.Address = part[0]
			cfg.Port, _ = strconv.Atoi(part[1])
		} else {
			cfg.Address = s
		}
	}
}

func WithAuthenticationPayload(payload map[string]any) ConfigOption {
	return func(cfg *Config) {
		cfg.authPayload = payload
	}
}

// -------------------- QUIC OPTIONS --------------------

const (
	configKeyQuicConfig           = "quic_config"
	configKeyQuicClientBufferSize = "quic_client_buffer_size"
)

func WithQuicClientConfig(c *quic.Config) ConfigOption {
	return func(cfg *Config) {
		if cfg.Type != TypeQuic {
			config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger).
				Errorf(context.Background(), "WithQuicClientConfig is not working for non-quic client")
			return
		}
		cfg.Set(configKeyQuicConfig, c)
	}
}

// WithQuicClientBufferSize set the buffer size of quic client
// total=mtu*size, default 1024
func WithQuicClientBufferSize(size int) ConfigOption {
	return func(cfg *Config) {
		if cfg.Type != TypeQuic {
			config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger).
				Errorf(context.Background(), "WithQuicClientBufferSize is not working for non-quic client")
			return
		}
		cfg.Set(configKeyQuicClientBufferSize, size)
	}
}
