package quota

const (
	ResourceTypeDataTrafficMegabytes = "data_traffic_megabytes"
	ResourceTypeDataTrafficGigabytes = "data_traffic_gigabytes"
	ResourceTypeDataTrafficTerabytes = "data_traffic_terabytes"
)

const (
	PeriodTypeDay   = "day"
	PeriodTypeMonth = "month"
)

const (
	TargetTypeDevice  = "device"
	TargetTypeNetwork = "network"
)

const (
	IdNoLimit = 0
)
