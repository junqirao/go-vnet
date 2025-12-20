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
	"go-vnet/common/router"
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
		auth.AuthorizedHandler
		router.Router

		Port    int
		Address string
		Type    Type
		MTU     int
	}
	ConfigOption func(cfg *Config)
)

// -------------------- OPTIONS --------------------

const (
	configKeyTLS    = "tls"
	configKeyLogger = "logger"
)

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
		cfg.Set(configKeyTLS, t)
	}
}

func WithLogger(l logger.Logger) ConfigOption {
	return func(cfg *Config) {
		cfg.Set(configKeyLogger, l)
	}
}

func WithAuthenticator(a auth.AuthorizedHandler) ConfigOption {
	return func(cfg *Config) {
		cfg.AuthorizedHandler = a
	}
}

func WithRouter(r router.Router) ConfigOption {
	return func(cfg *Config) {
		cfg.Router = r
	}
}

// -------------------- QUIC OPTIONS --------------------

const (
	configKeyQuicConfig = "quic_config"
)

func WithQuicConfig(c *quic.Config) ConfigOption {
	return func(cfg *Config) {
		if cfg.Type != TypeQuic {
			config.GetMappedConfig[logger.Logger](cfg, configKeyLogger,
				logger.DefaultLogger).
				Errorf(context.Background(), "WithQuicConfig is not working for non-quic server")
			return
		}
		cfg.Set(configKeyQuicConfig, c)
	}
}
