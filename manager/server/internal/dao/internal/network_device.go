// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// NetworkDeviceDao is the data access object for the table network_device.
type NetworkDeviceDao struct {
	table   string               // table is the underlying table name of the DAO.
	group   string               // group is the database configuration group name of the current DAO.
	columns NetworkDeviceColumns // columns contains all the column names of Table for convenient usage.
}

// NetworkDeviceColumns defines and stores column names for the table network_device.
type NetworkDeviceColumns struct {
	Id        string //
	DeviceId  string //
	NetworkId string //
	Quota     string //
	Settings  string //
}

// networkDeviceColumns holds the columns for the table network_device.
var networkDeviceColumns = NetworkDeviceColumns{
	Id:        "id",
	DeviceId:  "device_id",
	NetworkId: "network_id",
	Quota:     "quota",
	Settings:  "settings",
}

// NewNetworkDeviceDao creates and returns a new DAO object for table data access.
func NewNetworkDeviceDao() *NetworkDeviceDao {
	return &NetworkDeviceDao{
		group:   "default",
		table:   "network_device",
		columns: networkDeviceColumns,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *NetworkDeviceDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *NetworkDeviceDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *NetworkDeviceDao) Columns() NetworkDeviceColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *NetworkDeviceDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *NetworkDeviceDao) Ctx(ctx context.Context) *gdb.Model {
	return dao.DB().Model(dao.table).Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *NetworkDeviceDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
