package server

import (
	"context"
	"crypto/tls"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	"go-vnet/common/protocol"
)

var (
	defaultServerConfig = func() *Config {
		return &Config{
			MappedConfig: config.NewMappedConfig(),
		}
	}
)

const (
	TypeQuic Type = protocol.TransportTypeQuic
	TypeTCP  Type = protocol.TransportTypeTcp
)

type (
	Type   string
	Config struct {
		config.MappedConfig `yaml:"-" json:"-"`
		Servers             []*TransportConfig `yaml:"servers" json:"servers"`
		P2P                 *P2PConfig         `yaml:"p2p" json:"p2p"`
	}
	ConfigOption    func(cfg *Config)
	TransportConfig struct {
		config.MappedConfig `json:"-"`
		Name                string `yaml:"name" json:"name"`
		Port                int    `yaml:"port" json:"port"`
		Address             string `yaml:"address" json:"address"`
		Type                Type   `yaml:"type" json:"type"`
	}
	P2PConfig struct {
		Addresses []SignalingServerAddress `yaml:"addresses" json:"addresses"`
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
	ConfigKeyTLS = "tls"
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

// -------------------- QUIC OPTIONS --------------------

const (
	ConfigKeyQuicConfig = "quic_config"
)

func (t *TransportConfig) WithQuicConfig(c *quic.Config) *TransportConfig {
	if t.Type != TypeQuic {
		g.Log().Errorf(context.Background(), "WithQuicConfig is not working for non-quic transportServer")
		return t
	}
	t.Set(ConfigKeyQuicConfig, c)
	return t
}
