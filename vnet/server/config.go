package server

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/multiformats/go-multiaddr"
	"github.com/quic-go/quic-go"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
)

var (
	defaultServerConfig = func() *Config {
		return &Config{
			MappedConfig: config.NewMappedConfig(),
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
		Servers []*TransportConfig `json:"servers"`
		P2P     *P2PConfig         `json:"p2p"`
		MTU     int                `json:"mtu"`
		Auth    auth.Config        `json:"auth"`
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
	RelayConfig struct {
		IP        string `json:"ip"`
		Transport string `json:"transport"`
		Port      int    `json:"port"`
		Version   string `json:"version"` // only for quic
	}
)

func (t Type) String() string {
	return string(t)
}

func (c RelayConfig) MultiAddr(network ...string) multiaddr.Multiaddr {
	n := "ip4"
	if network != nil {
		n = network[0]
	}
	str := fmt.Sprintf("/%s/%s/%s/%d", n, c.IP, c.Transport, c.Port)
	if c.Version != "" {
		str += "/" + c.Version
	}
	return multiaddr.StringCast(str)
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
	ConfigKeyTLS           = "tls"
	ConfigKeyLogger        = "logger"
	ConfigKeyAuthChainFunc = "auth_chain_func"
)

func WithAuthConfig(a auth.Config) ConfigOption {
	return func(cfg *Config) {
		cfg.Auth = a
	}
}

func WithConfig(config *Config) ConfigOption {
	return func(cfg *Config) {
		cfg.MappedConfig = config.MappedConfig
		cfg.Servers = config.Servers
		cfg.MTU = config.MTU
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

func WithAuthenticationChainFunc(f ...auth.ServerAuthChainFunc) ConfigOption {
	return func(cfg *Config) {
		cfg.Set(ConfigKeyAuthChainFunc, f)
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
