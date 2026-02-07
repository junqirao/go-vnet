// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// Network is the golang structure of table network for DAO operations like Where/Data.
type Network struct {
	g.Meta      `orm:"table:network, do:true"`
	Id          interface{} //
	Name        interface{} //
	Cidr        interface{} //
	Mtu         interface{} //
	Quota       interface{} //
	Description interface{} //
	Extra       interface{} //
}
