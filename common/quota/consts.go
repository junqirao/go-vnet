package quota

const (
	ResourceTypeMegabytes          = "mb"
	ResourceTypeGigabytes          = "gb"
	ResourceTypeTerabytes          = "tb"
	ResourceTypeMegabytesPerSecond = "mbps"
)

const (
	PeriodTypeDay   = "day"
	PeriodTypeMonth = "month"
)

const (
	TargetTypeDeviceTraffic   = "device_traffic"
	TargetTypeDeviceBandwidth = "device_bandwidth"
	TargetTypeNetwork         = "network"
)

const (
	IdNoLimit = 0
)
