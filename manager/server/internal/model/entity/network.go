// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

// Network is the golang structure for table network.
type Network struct {
	Id          string `json:"id"          orm:"id"          ` //
	Name        string `json:"name"        orm:"name"        ` //
	Cidr        string `json:"cidr"        orm:"cidr"        ` //
	Mtu         int    `json:"mtu"         orm:"mtu"         ` //
	Description string `json:"description" orm:"description" ` //
	Extra       string `json:"extra"       orm:"extra"       ` //
}
