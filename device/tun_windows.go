package device

import (
	"github.com/songgao/water"
)

func newWaterDevice(config Config) (d *waterDevice, err error) {
	d = new(waterDevice)
	d.Interface, err = water.New(water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID: "tap0901",
			// InterfaceName: config.Name,
			Network: config.CIDR,
		},
	})
	return
}
