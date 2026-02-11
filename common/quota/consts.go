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
	PeriodPermanent = "permanent"
)

const (
	TargetTypeDevice  = "device"
	TargetTypeNetwork = "network"
)

const (
	IdNoLimit = 0
)
