package device

import (
	"errors"
	"runtime"
)

const (
	TypeWG    = "WireGuard"
	TypeWater = "Water"
)

type (
	Type string
	// Config ...
	Config struct {
		Type Type   `json:"type"`
		Name string `json:"name"`
		CIDR string `json:"cidr"`
		DNS  string `json:"dns"`
		MTU  int    `json:"mtu"`
	}
)

func (c *Config) check() (err error) {
	if c.Name == "" {
		c.Name = DefaultTunDeviceName
	}
	if c.CIDR == "" {
		err = errors.New("cidr not set")
	}
	if c.Type == "" {
		if runtime.GOOS == "linux" {
			c.Type = TypeWater
		} else {
			c.Type = TypeWG
		}
	}
	return
}
