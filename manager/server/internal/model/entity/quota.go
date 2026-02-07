// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

// Quota is the golang structure for table quota.
type Quota struct {
	Id         int    `json:"id"         orm:"id"          ` //
	Name       string `json:"name"       orm:"name"        ` //
	Type       string `json:"type"       orm:"type"        ` //
	Value      int    `json:"value"      orm:"value"       ` //
	Period     string `json:"period"     orm:"period"      ` //
	Target     string `json:"target"     orm:"target"      ` //
	TargetType string `json:"targetType" orm:"target_type" ` //
}
