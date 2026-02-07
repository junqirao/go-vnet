// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// QuotaFlowDao is the data access object for the table quota_flow.
type QuotaFlowDao struct {
	table   string           // table is the underlying table name of the DAO.
	group   string           // group is the database configuration group name of the current DAO.
	columns QuotaFlowColumns // columns contains all the column names of Table for convenient usage.
}

// QuotaFlowColumns defines and stores column names for the table quota_flow.
type QuotaFlowColumns struct {
	Id          string //
	Usage       string //
	Target      string //
	TargetType  string //
	RecordStart string //
	RecordEnd   string //
}

// quotaFlowColumns holds the columns for the table quota_flow.
var quotaFlowColumns = QuotaFlowColumns{
	Id:          "id",
	Usage:       "usage",
	Target:      "target",
	TargetType:  "target_type",
	RecordStart: "record_start",
	RecordEnd:   "record_end",
}

// NewQuotaFlowDao creates and returns a new DAO object for table data access.
func NewQuotaFlowDao() *QuotaFlowDao {
	return &QuotaFlowDao{
		group:   "default",
		table:   "quota_flow",
		columns: quotaFlowColumns,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *QuotaFlowDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *QuotaFlowDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *QuotaFlowDao) Columns() QuotaFlowColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *QuotaFlowDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *QuotaFlowDao) Ctx(ctx context.Context) *gdb.Model {
	return dao.DB().Model(dao.table).Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *QuotaFlowDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
