package device

const (
	DefaultMTU = 1392
)

type DispatchedDeviceInfo struct {
	Name string `json:"name"`
	CIDR string `json:"cidr"`
	MTU  int    `json:"mtu"`
}
