// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OpsSystemMetricsDao is the data access object for the table ops_system_metrics.
type OpsSystemMetricsDao struct {
	table    string                  // table is the underlying table name of the DAO.
	group    string                  // group is the database configuration group name of the current DAO.
	columns  OpsSystemMetricsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler      // handlers for customized model modification.
}

// OpsSystemMetricsColumns defines and stores column names for the table ops_system_metrics.
type OpsSystemMetricsColumns struct {
	Id          string // 主键ID
	MetricType  string // 指标类型：cpu/memory/disk/network/runtime/db_pool/redis_pool
	MetricData  string // 指标数据（JSONB，结构因类型而异）
	CollectedAt string // 采集时间
}

// opsSystemMetricsColumns holds the columns for the table ops_system_metrics.
var opsSystemMetricsColumns = OpsSystemMetricsColumns{
	Id:          "id",
	MetricType:  "metric_type",
	MetricData:  "metric_data",
	CollectedAt: "collected_at",
}

// NewOpsSystemMetricsDao creates and returns a new DAO object for table data access.
func NewOpsSystemMetricsDao(handlers ...gdb.ModelHandler) *OpsSystemMetricsDao {
	return &OpsSystemMetricsDao{
		group:    "default",
		table:    "ops_system_metrics",
		columns:  opsSystemMetricsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OpsSystemMetricsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OpsSystemMetricsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OpsSystemMetricsDao) Columns() OpsSystemMetricsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OpsSystemMetricsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OpsSystemMetricsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OpsSystemMetricsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
