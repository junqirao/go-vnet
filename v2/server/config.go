package server

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
	defaultServerConfig = func() *Config {
		return &Config{
			MappedConfig: config.NewMappedConfig(),
			Port:         9800,
			Address:      "0.0.0.0",
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

		Name    string      `json:"name"`
		Port    int         `json:"port"`
		Address string      `json:"address"`
		Type    Type        `json:"type"`
		MTU     int         `json:"mtu"`
		Auth    auth.Config `json:"auth"`
	}
	ConfigOption func(cfg *Config)
)

func NewTransportServerConfig(opts ...ConfigOption) *Config {
	cfg := defaultServerConfig()
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

func WithName(s string) ConfigOption {
	return func(cfg *Config) {
		cfg.Name = s
	}
}

func WithAuthConfig(a auth.Config) ConfigOption {
	return func(cfg *Config) {
		cfg.Auth = a
	}
}

func WithConfig(config *Config) ConfigOption {
	return func(cfg *Config) {
		cfg.MappedConfig = config.MappedConfig
		cfg.Port = config.Port
		cfg.Address = config.Address
		cfg.Type = config.Type
		cfg.MTU = config.MTU
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

func WithTLSConfig(t *tls.Config) ConfigOption {
	return func(cfg *Config) {
		cfg.Set(ConfigKeyTLS, t)
	}
}

func WithLogger(l logger.Logger) ConfigOption {
	return func(cfg *Config) {
		cfg.Set(ConfigKeyLogger, l)
	}
}

// -------------------- QUIC OPTIONS --------------------

const (
	ConfigKeyQuicConfig = "quic_config"
)

func WithQuicConfig(c *quic.Config) ConfigOption {
	return func(cfg *Config) {
		if cfg.Type != TypeQuic {
			config.GetMappedConfig[logger.Logger](cfg, ConfigKeyLogger,
				logger.DefaultLogger).
				Errorf(context.Background(), "WithQuicConfig is not working for non-quic transportServer")
			return
		}
		cfg.Set(ConfigKeyQuicConfig, c)
	}
}
