package server

import (
	"context"
	"crypto/tls"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	"go-vnet/common/logger"
)

var (
	defaultServerConfig = func() *Config {
		return &Config{
			MappedConfig: config.NewMappedConfig(),
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
		Servers []*TransportConfig `json:"servers"`
		P2P     *P2PConfig         `json:"p2p"`
	}
	ConfigOption    func(cfg *Config)
	TransportConfig struct {
		config.MappedConfig
		Name    string `json:"name"`
		Port    int    `json:"port"`
		Address string `json:"address"`
		Type    Type   `json:"type"`
	}
	P2PConfig struct {
		Addresses []SignalingServerAddress `json:"addresses"`
	}
	NetworkLink struct {
		SubDeviceId uint64 `json:"sub_device_id"`
		Key         string `json:"key"`
	}
)

func (t Type) String() string {
	return string(t)
}

func NewConfig(opts ...ConfigOption) *Config {
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

func WithConfig(config *Config) ConfigOption {
	return func(cfg *Config) {
		cfg.MappedConfig = config.MappedConfig
		cfg.Servers = config.Servers
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

func (t *TransportConfig) WithQuicConfig(c *quic.Config) *TransportConfig {
	if t.Type != TypeQuic {
		config.GetMappedConfig[logger.Logger](t, ConfigKeyLogger,
			logger.DefaultLogger).
			Errorf(context.Background(), "WithQuicConfig is not working for non-quic transportServer")
		return t
	}
	t.Set(ConfigKeyQuicConfig, c)
	return t
}
