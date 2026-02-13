// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
)

// QuotaFlow is the golang structure of table quota_flow for DAO operations like Where/Data.
type QuotaFlow struct {
	g.Meta      `orm:"table:quota_flow, do:true"`
	Id          interface{} //
	Quota       interface{} //
	Usage       interface{} //
	Target      interface{} //
	TargetType  interface{} //
	RecordStart interface{} //
	RecordEnd   interface{} //
}
