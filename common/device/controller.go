package device

import "go-vnet/common/logger"

type option func(c *Controller)

type Controller struct {
	TunDevice
	config        Config
	clearCIDRFunc func()
	logger        logger.Logger
}

var (
	// WithLogger set logger to current device
	WithLogger = func(l logger.Logger) option {
		return func(c *Controller) {
			c.logger = l
		}
	}
	// WithConfig set config to current device
	WithConfig = func(config Config) option {
		return func(c *Controller) {
			c.config = config
		}
	}
	WithTunDevice = func(dev TunDevice) option {
		return func(c *Controller) {
			c.TunDevice = dev
		}
	}

	defaultOptions = []option{WithLogger(logger.DefaultLogger)}
)

func NewController(opts ...option) *Controller {
	c := new(Controller)
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Setup ...
func (c *Controller) Setup() (err error) {
	if err = c.config.check(); err != nil {
		return
	}

	switch c.config.Type {
	case TypeWater:
		c.TunDevice, err = newWaterDevice(c.config)
		if err != nil {
			return
		}
		c.config.Name = c.Name()
	default:
		c.TunDevice, err = newWireGuardDevice(c.config)
		if err != nil {
			return
		}
	}

	err = c.SetupProperties()
	return
}

func (c *Controller) GetConfig() Config {
	return c.config
}
