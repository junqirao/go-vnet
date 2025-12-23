package client

import (
	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/server/auth"
)

type Config struct {
	config.MappedConfig
	NetworkId          string      `json:"network_id"`
	Server             string      `json:"server"`
	InsecureSkipVerify bool        `json:"insecure_skip_verify"`
	Auth               auth.Config `json:"auth"`
}

type ConfigOption func(cfg *Config)

func WithNetworkId(id string) ConfigOption {
	return func(cfg *Config) {
		cfg.NetworkId = id
	}
}

func WithServer(server string) ConfigOption {
	return func(cfg *Config) {
		cfg.Server = server
	}
}

func WithAuth(auth auth.Config) ConfigOption {
	return func(cfg *Config) {
		cfg.Auth = auth
	}
}

func WithLogger(l logger.Logger) ConfigOption {
	return func(cfg *Config) {
		cfg.Set(configKeyLogger, l)
	}
}

func WithInsecureSkipVerify(b bool) ConfigOption {
	return func(cfg *Config) {
		cfg.InsecureSkipVerify = b
	}
}

func NewConfig(opts ...ConfigOption) Config {
	c := Config{
		MappedConfig: config.NewMappedConfig(),
	}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}
