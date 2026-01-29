package client

import (
	"context"
	"crypto/tls"
	"strconv"
	"strings"

	"github.com/quic-go/quic-go"

	"go-vnet/common/auth"
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
	TypeTCP  Type = "tcp"
)

type (
	Type   string
	Config struct {
		config.MappedConfig
		authPayload map[string]any

		NetworkId          string      `yaml:"network_id" json:"network_id"`
		Port               int         `yaml:"port" json:"port"`
		Address            string      `yaml:"address" json:"address"`
		Type               Type        `yaml:"type" json:"type"`
		MTU                int         `yaml:"mtu" json:"mtu"`
		InsecureSkipVerify bool        `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
		Auth               auth.Config `yaml:"auth" json:"auth"`
		P2P                P2PConfig   `yaml:"p2p" json:"p2p"`
	}
	P2PConfig struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
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
	ConfigKeyTLS    = "tls"
	ConfigKeyLogger = "logger"
)

func WithTLSConfig(t *tls.Config) ConfigOption {
	return func(cfg *Config) {
		cfg.Set(ConfigKeyTLS, t)
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

func WithInsecureSkipVerify(b bool) ConfigOption {
	return func(cfg *Config) {
		cfg.InsecureSkipVerify = b
	}
}

// -------------------- QUIC OPTIONS --------------------

const (
	ConfigKeyQuicConfig           = "quic_config"
	ConfigKeyQuicClientBufferSize = "quic_client_buffer_size"
)

func WithQuicClientConfig(c *quic.Config) ConfigOption {
	return func(cfg *Config) {
		if cfg.Type != TypeQuic {
			config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger).
				Errorf(context.Background(), "WithQuicClientConfig is not working for non-quic client")
			return
		}
		cfg.Set(ConfigKeyQuicConfig, c)
	}
}

// WithQuicClientBufferSize set the buffer size of quic client
// total=mtu*size, default 1024
func WithQuicClientBufferSize(size int) ConfigOption {
	return func(cfg *Config) {
		if cfg.Type != TypeQuic {
			config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger, logger.DefaultLogger).
				Errorf(context.Background(), "WithQuicClientBufferSize is not working for non-quic client")
			return
		}
		cfg.Set(ConfigKeyQuicClientBufferSize, size)
	}
}
