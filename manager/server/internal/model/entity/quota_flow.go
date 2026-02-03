// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// QuotaFlow is the golang structure for table quota_flow.
type QuotaFlow struct {
	Id          int         `json:"id"          orm:"id"           ` //
	Usage       int         `json:"usage"       orm:"usage"        ` //
	Target      string      `json:"target"      orm:"target"       ` //
	TargetType  string      `json:"targetType"  orm:"target_type"  ` //
	RecordStart *gtime.Time `json:"recordStart" orm:"record_start" ` //
	RecordEnd   *gtime.Time `json:"recordEnd"   orm:"record_end"   ` //
}
