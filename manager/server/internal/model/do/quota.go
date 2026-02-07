// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// Quota is the golang structure of table quota for DAO operations like Where/Data.
type Quota struct {
	g.Meta     `orm:"table:quota, do:true"`
	Id         interface{} //
	Name       interface{} //
	Type       interface{} //
	Value      interface{} //
	Period     interface{} //
	Target     interface{} //
	TargetType interface{} //
}
