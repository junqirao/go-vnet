package transport

import (
	"context"
	"crypto/tls"
	"strconv"
	"strings"

	"github.com/quic-go/quic-go"

	"go-vnet/common/auth"
	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/common/router"
)

var (
	defaultServerConfig = func() *ServerConfig {
		return &ServerConfig{
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
	Type         string
	ServerConfig struct {
		config.MappedConfig
		auth.AuthorizedHandler
		router.Router

		Name           string `json:"name"`
		Port           int    `json:"port"`
		Address        string `json:"address"`
		Type           Type   `json:"type"`
		MTU            int    `json:"mtu"`
		AuthType       string `json:"auth_type"`
		AuthPassword   string `json:"auth_password,omitempty"`
		AuthPublicKey  string `json:"auth_public_key,omitempty"`
		AuthPrivateKey string `json:"auth_private_key,omitempty"`
	}
	ServerConfigOption func(cfg *ServerConfig)
)

func NewTransportServerConfig(opts ...ServerConfigOption) *ServerConfig {
	cfg := defaultServerConfig()
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

func WithName(s string) ServerConfigOption {
	return func(cfg *ServerConfig) {
		cfg.Name = s
	}
}

func WithAuthType(s string) ServerConfigOption {
	return func(cfg *ServerConfig) {
		cfg.AuthType = s
	}
}

func WithConfig(config *ServerConfig) ServerConfigOption {
	return func(cfg *ServerConfig) {
		cfg.MappedConfig = config.MappedConfig
		cfg.Port = config.Port
		cfg.Address = config.Address
		cfg.Type = config.Type
		cfg.MTU = config.MTU
	}
}

func WithAddress(s string) ServerConfigOption {
	return func(cfg *ServerConfig) {
		part := strings.Split(s, ":")
		if len(part) > 1 {
			cfg.Address = part[0]
			cfg.Port, _ = strconv.Atoi(part[1])
		} else {
			cfg.Address = s
		}
	}
}

func WithTLSConfig(t *tls.Config) ServerConfigOption {
	return func(cfg *ServerConfig) {
		cfg.Set(configKeyTLS, t)
	}
}

func WithLogger(l logger.Logger) ServerConfigOption {
	return func(cfg *ServerConfig) {
		cfg.Set(configKeyLogger, l)
	}
}

func WithAuthenticator(a auth.AuthorizedHandler) ServerConfigOption {
	return func(cfg *ServerConfig) {
		cfg.AuthorizedHandler = a
	}
}

func WithRouter(r router.Router) ServerConfigOption {
	return func(cfg *ServerConfig) {
		cfg.Router = r
	}
}

// -------------------- QUIC OPTIONS --------------------

const (
	configKeyQuicConfig = "quic_config"
)

func WithQuicConfig(c *quic.Config) ServerConfigOption {
	return func(cfg *ServerConfig) {
		if cfg.Type != TypeQuic {
			config.GetMappedConfig[logger.Logger](cfg, configKeyLogger,
				logger.DefaultLogger).
				Errorf(context.Background(), "WithQuicConfig is not working for non-quic transportServer")
			return
		}
		cfg.Set(configKeyQuicConfig, c)
	}
}
