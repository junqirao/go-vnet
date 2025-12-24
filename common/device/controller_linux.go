package device

import (
	"os/exec"
	"strconv"
)

// setup ...
func (c *controller) setup() error {
	err := c.setCIDR(c.config.CIDR)
	if err != nil {
		return err
	}
	err = c.setMtu()
	if err != nil {
		return err
	}
	err = c.Up()
	if err != nil {
		return err
	}
	// if d.config.DNS != "" {
	// _ = log.Warn("device.dns config only work in windows value is ignored : ", d.DNS)
	// _ = log.Warn("you need to setup dns manually")
	// }
	return nil
}

// setCIDR ...
func (c *controller) setCIDR(cidr string) error {
	name := c.Name()
	cmd := exec.Command("/sbin/ip", "address", "add", cidr, "dev", name)
	err := cmd.Run()
	if err == nil {
		c.clearCIDRFunc = func() {
			cmd := exec.Command("/sbin/ip", "address", "del", cidr, "dev", name)
			_ = cmd.Run()
		}
		c.config.CIDR = cidr
	}
	return nil
}

// setMtu ...
func (c *controller) setMtu() error {
	name := c.Name()
	cmd := exec.Command("/sbin/ip", "link", "set", "dev", name, "mtu", strconv.Itoa(c.config.MTU))
	_ = cmd.Run()
	return nil
}

// OverwriteCIDR of device
func (c *controller) OverwriteCIDR(cidr string) error {
	if cidr == c.config.CIDR {
		return nil
	}
	if c.clearCIDRFunc != nil {
		c.clearCIDRFunc()
	}
	return c.setCIDR(cidr)
}

// OverwriteMTU of device
func (c *controller) OverwriteMTU(mtu int) error {
	c.config.MTU = mtu
	return c.setMtu()
}

// Up ...
func (c *controller) Up() error {
	cmd := exec.Command("/sbin/ip", "link", "set", "dev", c.Name(), "up")
	return cmd.Run()
}

// Down ...
func (c *controller) Down() error {
	cmd := exec.Command("/sbin/ip", "link", "set", "dev", c.Name(), "down")
	return cmd.Run()
}
