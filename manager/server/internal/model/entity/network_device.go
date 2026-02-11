// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

// NetworkDevice is the golang structure for table network_device.
type NetworkDevice struct {
	Id        int    `json:"id"        orm:"id"         ` //
	DeviceId  string `json:"deviceId"  orm:"device_id"  ` //
	NetworkId string `json:"networkId" orm:"network_id" ` //
	Quota     int    `json:"quota"     orm:"quota"      ` //
	Bandwidth int    `json:"bandwidth" orm:"bandwidth"  ` // quota id
	Settings  string `json:"settings"  orm:"settings"   ` //
}
