package server

import (
	"context"
	"fmt"

	"go-vnet/common/config"
	"go-vnet/common/logger"
	"go-vnet/server/transport"
)

type (
	Server struct {
		sig               chan struct{}
		logger            logger.Logger
		cfg               *Config
		transportServers  []transport.Server
		funcCallEventChan chan *transport.FuncCallEvent
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
		sig:               make(chan struct{}),
		cfg:               cfg,
		logger:            config.GetMappedConfig[logger.Logger](cfg, configKeyLogger, logger.DefaultLogger),
		funcCallEventChan: make(chan *transport.FuncCallEvent, 1024),
	}
}

func (s *Server) Run(ctx context.Context) (err error) {
	// run transport servers
	for i, cfg := range s.cfg.Transports {
		if cfg.Name == "" {
			cfg.Name = fmt.Sprintf("unnamed_transport_server_%d", i)
		}
		cfg.Set(configKeyLogger, s.logger)

		// new transport server
		server := transport.NewTransportServer(s.funcCallEventChan, cfg)
		s.transportServers = append(s.transportServers, server)

		// run transport server
		go func() {
			ctx = context.WithValue(ctx, "server", server)
			ctx = context.WithValue(ctx, "transport_server", cfg.Name)
			if err = server.Serve(ctx); err != nil {
				s.logger.Errorf(ctx, "run transport server %s error: %v", cfg.Type, err)
				return
			}
			s.logger.Infof(ctx, "transport server %s closed", cfg.Type)
		}()
	}

	// run func call server
	go s.processFuncCallLoop(ctx)

	// wait for signal
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
