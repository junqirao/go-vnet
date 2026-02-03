// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// QuotaDao is the data access object for the table quota.
type QuotaDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  QuotaColumns       // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// QuotaColumns defines and stores column names for the table quota.
type QuotaColumns struct {
	Id          string //
	Name        string //
	Type        string //
	Value       string //
	PeriodStart string //
	PeriodEnd   string //
	Target      string //
	TargetType  string //
}

// quotaColumns holds the columns for the table quota.
var quotaColumns = QuotaColumns{
	Id:          "id",
	Name:        "name",
	Type:        "type",
	Value:       "value",
	PeriodStart: "period_start",
	PeriodEnd:   "period_end",
	Target:      "target",
	TargetType:  "target_type",
}

// NewQuotaDao creates and returns a new DAO object for table data access.
func NewQuotaDao(handlers ...gdb.ModelHandler) *QuotaDao {
	return &QuotaDao{
		group:    "default",
		table:    "quota",
		columns:  quotaColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *QuotaDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *QuotaDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *QuotaDao) Columns() QuotaColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *QuotaDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *QuotaDao) Ctx(ctx context.Context) *gdb.Model {
	model := dao.DB().Model(dao.table)
	for _, handler := range dao.handlers {
		model = handler(model)
	}
	return model.Safe().Ctx(ctx)
}

// Transaction wraps the transaction logic using function f.
// It rolls back the transaction and returns the error if function f returns a non-nil error.
// It commits the transaction and returns nil if function f returns nil.
//
// Note: Do not commit or roll back the transaction in function f,
// as it is automatically handled by this function.
func (dao *QuotaDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
