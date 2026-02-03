// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Quota is the golang structure of table quota for DAO operations like Where/Data.
type Quota struct {
	g.Meta      `orm:"table:quota, do:true"`
	Id          any         //
	Name        any         //
	Type        any         //
	Value       any         //
	PeriodStart *gtime.Time //
	PeriodEnd   *gtime.Time //
	Target      any         //
	TargetType  any         //
}
