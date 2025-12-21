package server

import (
	"context"
	"fmt"
	"sync"

	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/server/transport"
)

type (
	Server struct {
		sig              chan struct{}
		logger           logger.Logger
		cfg              *Config
		transportServers []transport.Server
		mu               sync.RWMutex
		networks         map[string]*Network
	}
	Config struct {
		config.MappedConfig
		Transports []*transport.ServerConfig `json:"transport"`
	}
	ConfigOption func(cfg *Config)
)

const (
	configKeyLogger = "logger"
)

func WithLogger(logger logger.Logger) ConfigOption {
	return func(cfg *Config) {
		cfg.Set(configKeyLogger, logger)
	}
}

func NewConfig(opts ...ConfigOption) *Config {
	cfg := &Config{
		MappedConfig: config.NewMappedConfig(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func NewServer(cfg *Config) *Server {
	return &Server{
		sig:      make(chan struct{}),
		cfg:      cfg,
		networks: map[string]*Network{},
		logger:   config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger),
	}
}

func (s *Server) RegisterNetwork(_ context.Context, network *Network) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.networks[network.ID] = network
}

func (s *Server) Run(ctx context.Context) (err error) {
	// run transport servers
	for i, cfg := range s.cfg.Transports {
		if cfg.Name == "" {
			cfg.Name = fmt.Sprintf("unnamed_transport_server_%d", i)
		}
		cfg.Set(configKeyLogger, s.logger)
		server := transport.NewTransportServer(cfg)
		s.transportServers = append(s.transportServers, server)
		go func() {
			ctx = context.WithValue(ctx, "server", server)
			ctx = context.WithValue(ctx, "transport_server", cfg.Name)
			if err = server.Serve(ctx); err != nil {
				return
			}
			s.logger.Infof(ctx, "transport server %s closed", cfg.Type)
		}()
	}
	select {
	case <-s.sig:
		s.logger.Infof(ctx, "server closed")
		for _, server := range s.transportServers {
			_ = server.Close()
		}
		return
	case <-ctx.Done():
		return ctx.Err()
	}
}
