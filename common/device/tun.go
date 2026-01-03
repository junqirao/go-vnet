package device

const DefaultTunDeviceName = "default-tun-device"

type (
	// IDevice ...
	IDevice interface {
		Name() string                           // return device name if device exists
		Setup() error                           // create device
		Close() error                           // close device
		OverwriteCIDR(cidr string) error        // overwrite cidr
		OverwriteMTU(mtu int) error             // overwrite mtu
		Up() error                              // set device up
		Down() error                            // set device down
		Read(packet []byte) (n int, err error)  // read
		Write(packet []byte) (n int, err error) // write
		GetConfig() Config
	}
	// tunDevice ...
	tunDevice interface {
		Name() string                           // return device name if device exists
		Close() error                           // close device
		Read(packet []byte) (n int, err error)  // read
		Write(packet []byte) (n int, err error) // write
	}
)

// NewTunDevice ...
func NewTunDevice(cfg Config, opts ...option) (device IDevice) {
	options := append(defaultOptions, WithConfig(cfg))
	d := newController(append(options, opts...)...)
	d.config = cfg
	return d
}
