// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

// QuotaFlow is the golang structure for table quota_flow.
type QuotaFlow struct {
	Id          int    `json:"id"          orm:"id"           ` //
	Quota       int    `json:"quota"       orm:"quota"        ` //
	Usage       int    `json:"usage"       orm:"usage"        ` //
	Target      string `json:"target"      orm:"target"       ` //
	TargetType  string `json:"targetType"  orm:"target_type"  ` //
	RecordStart int64  `json:"recordStart" orm:"record_start" ` //
	RecordEnd   int64  `json:"recordEnd"   orm:"record_end"   ` //
}
