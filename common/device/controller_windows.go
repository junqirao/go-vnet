package device

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
)

// SetupProperties ...
func (c *Controller) SetupProperties() error {
	err := c.setCIDR(c.config.CIDR)
	if err != nil {
		return fmt.Errorf("set cidr error: %w", err)
	}
	err = c.setMTU(c.config.MTU)
	if err != nil {
		return fmt.Errorf("set mtu error: %w", err)
	}
	err = c.setDNS(c.config.DNS)
	if err != nil {
		return fmt.Errorf("set dns error: %w", err)
	}
	// auto up in windows
	return nil
}

// setCIDR ...
func (c *Controller) setCIDR(cidr string) error {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	name := c.Name()
	cmd := exec.Command("PowerShell",
		"netsh", "interface", "ip", "set", "Address", "Name=\""+name+"\"", "source=static", "addr="+ip.String(), "mask="+ipv4MaskString(ipNet.Mask), "store=active")
	err = cmd.Run()
	if err == nil {
		c.clearCIDRFunc = func() {
			cmd := exec.Command("PowerShell",
				"netsh", "interface", "ip", "delete", "Address", "Name=\""+name+"\"", "addr="+ip.String())
			_ = cmd.Run()
		}
		c.config.CIDR = cidr
	}
	return err
}

// setDNS ...
func (c *Controller) setDNS(dns string) error {
	if dns == "" {
		return nil
	}
	// netsh interface ip  set dnsservers Name="tunnel" source=static address=223.5.5.5 register=primary validate=no
	name := c.Name()
	cmd := exec.Command("PowerShell",
		"netsh", "interface", "ip", "set", "dnsservers", "Name=\""+name+"\"", "source=static", "address="+dns, "register=primary", "validate=no")
	return cmd.Run()
}

// setMTU ...
func (c *Controller) setMTU(mtu int) error {
	name := c.Name()
	cmd := exec.Command("PowerShell",
		"netsh", "interface", "ipv4", "set", "interface", "\""+name+"\"", "mtu="+strconv.Itoa(mtu))
	return cmd.Run()
}

// OverwriteCIDR of device
func (c *Controller) OverwriteCIDR(cidr string) error {
	if cidr == c.config.CIDR {
		return nil
	}
	if c.clearCIDRFunc != nil {
		c.clearCIDRFunc()
	}
	return c.setCIDR(cidr)
}

// OverwriteMTU of device
func (c *Controller) OverwriteMTU(mtu int) error {
	c.config.MTU = mtu
	return c.setMTU(mtu)
}

// Up ...
func (c *Controller) Up() error {
	cmd := exec.Command("PowerShell", "netsh", "interface", "set", "interface", c.Name(), "enabled")
	return cmd.Run()
}

// Down ...
func (c *Controller) Down() error {
	cmd := exec.Command("PowerShell", "netsh", "interface", "set", "interface", c.Name(), "disabled")
	return cmd.Run()
}
