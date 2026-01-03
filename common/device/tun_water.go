package device

import (
	"github.com/songgao/water"
)

type (
	// waterDevice ...
	// supports only windows,linux,osx
	waterDevice struct {
		*water.Interface
	}
)
