package device

import "go-vnet/common/logger"

type option func(c *controller)

type controller struct {
	tunDevice
	config        Config
	clearCIDRFunc func()
	logger        logger.Logger
}

var (
	// WithLogger set logger to current device
	WithLogger = func(l logger.Logger) option {
		return func(c *controller) {
			c.logger = l
		}
	}
	// WithConfig set config to current device
	WithConfig = func(config Config) option {
		return func(c *controller) {
			c.config = config
		}
	}

	defaultOptions = []option{WithLogger(logger.DefaultLogger)}
)

func newController(opts ...option) *controller {
	c := new(controller)
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Setup ...
func (c *controller) Setup() (err error) {
	if err = c.config.check(); err != nil {
		return
	}

	switch c.config.Type {
	case TypeWater:
		c.tunDevice, err = newWaterDevice(c.config)
		if err != nil {
			return
		}
		c.config.Name = c.Name()
	default:
		c.tunDevice, err = newWireGuardDevice(c.config)
		if err != nil {
			return
		}
	}

	err = c.setup()
	return
}
