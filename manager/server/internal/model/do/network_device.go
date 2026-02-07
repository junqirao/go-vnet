// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// NetworkDevice is the golang structure of table network_device for DAO operations like Where/Data.
type NetworkDevice struct {
	g.Meta    `orm:"table:network_device, do:true"`
	Id        interface{} //
	DeviceId  interface{} //
	NetworkId interface{} //
	Quota     interface{} //
	Settings  interface{} //
}
