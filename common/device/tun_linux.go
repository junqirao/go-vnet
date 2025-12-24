package device

import (
	"github.com/songgao/water"
)

func newWaterDevice(config Config) (d *waterDevice, err error) {
	d = new(waterDevice)
	d.Interface, err = water.New(water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: config.Name,
		},
	})
	return
}
