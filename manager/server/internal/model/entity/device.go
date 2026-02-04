// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// Device is the golang structure for table device.
type Device struct {
	Id         string      `json:"id"         orm:"id"          ` //
	Name       string      `json:"name"       orm:"name"        ` //
	Enabled    int         `json:"enabled"    orm:"enabled"     ` //
	PublicKey  string      `json:"publicKey"  orm:"public_key"  ` //
	PrivateKey string      `json:"privateKey" orm:"private_key" ` //
	CreatedAt  *gtime.Time `json:"createdAt"  orm:"created_at"  ` //
}
