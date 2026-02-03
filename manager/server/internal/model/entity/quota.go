// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Quota is the golang structure for table quota.
type Quota struct {
	Id          int         `json:"id"          orm:"id"           ` //
	Name        string      `json:"name"        orm:"name"         ` //
	Type        string      `json:"type"        orm:"type"         ` //
	Value       int         `json:"value"       orm:"value"        ` //
	PeriodStart *gtime.Time `json:"periodStart" orm:"period_start" ` //
	PeriodEnd   *gtime.Time `json:"periodEnd"   orm:"period_end"   ` //
	Target      string      `json:"target"      orm:"target"       ` //
	TargetType  string      `json:"targetType"  orm:"target_type"  ` //
}
