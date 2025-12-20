package device

import (
	"net"
	"testing"
)

func TestGetInterfaces(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil {
		t.Fatal(err)
		return
	}
	for _, v := range interfaces {
		t.Logf("%+v", v)
	}
}

func TestNewTunDevice(t *testing.T) {
	device := NewTunDevice(Config{
		Name: "test-device",
		CIDR: "192.168.4.2/32",
		MTU:  1500,
	})
	err := device.Setup()
	if err != nil {
		t.Fatal(err)
		return
	}
}
