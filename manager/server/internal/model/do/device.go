// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// Device is the golang structure of table device for DAO operations like Where/Data.
type Device struct {
	g.Meta     `orm:"table:device, do:true"`
	Id         any         //
	Name       any         //
	Key        any         //
	Enabled    any         //
	PublicKey  any         //
	PrivateKey any         //
	CreatedAt  *gtime.Time //
}
