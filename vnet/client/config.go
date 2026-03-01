package client

import (
	"context"
	"crypto/tls"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
)

var (
	defaultClientConfig = func() *Config {
		return &Config{
			MappedConfig: config.NewMappedConfig(),
			Type:         TypeQuic,
			P2P: P2PConfig{
				Enabled:        true,
				ListenAddr:     []string{},
				TryInterval:    30,
				AddressRefresh: 120, // 默认2分钟刷新一次
				ActiveDialPeer: false,
			},
			Encrypt:  true,
			Compress: false,
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
		config.MappedConfig `json:"-"`

		Link               string        `yaml:"link" json:"link"`
		PublicKey          string        `yaml:"public_key" json:"public_key"`
		Port               int           `yaml:"port" json:"port"`
		Address            string        `yaml:"address" json:"address"`
		Type               Type          `yaml:"type" json:"type"`
		InsecureSkipVerify bool          `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
		P2P                P2PConfig     `yaml:"p2p" json:"p2p"`
		Compress           bool          `yaml:"compress" json:"compress"`
		Encrypt            bool          `yaml:"encrypt" json:"encrypt"`
		Manager            ManagerConfig `yaml:"manager" json:"manager"`
		DeviceMode         string        `yaml:"device_mode" json:"device_mode"`
	}
	ManagerConfig struct {
		Enabled bool   `yaml:"enabled" json:"enabled"`
		Listen  string `yaml:"listen" json:"listen"`
		Port    int    `yaml:"port" json:"port"`
	}
	P2PConfig struct {
		Enabled        bool              `yaml:"enabled" json:"enabled"`
		ListenAddr     []string          `yaml:"listen_addr" json:"listen_addr"`
		TryInterval    int               `yaml:"try_interval" json:"try_interval"`       // 尝试P2P连接的间隔（秒）
		AddressRefresh int               `yaml:"address_refresh" json:"address_refresh"` // 地址刷新间隔（秒），默认120秒（2分钟）
		ActiveDialPeer bool              `yaml:"active_dial_peer" json:"active_dial_peer"`
		ICEServers     []ICEServerConfig `yaml:"ice_servers" json:"ice_servers"` // STUN/TURN服务器配置
	}
	ICEServerConfig struct {
		URLs       []string `yaml:"urls" json:"urls"`             // STUN/TURN服务器URL
		Username   string   `yaml:"username" json:"username"`     // TURN服务器用户名（可选）
		Credential string   `yaml:"credential" json:"credential"` // TURN服务器密码（可选）
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
			g.Log().Errorf(context.Background(), "WithQuicClientConfig is not working for non-quic client")
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
			g.Log().Errorf(context.Background(), "WithQuicClientBufferSize is not working for non-quic client")
			return
		}
		cfg.Set(ConfigKeyQuicClientBufferSize, size)
	}
}
