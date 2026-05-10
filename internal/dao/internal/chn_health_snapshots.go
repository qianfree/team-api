// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChnHealthSnapshotsDao is the data access object for the table chn_health_snapshots.
type ChnHealthSnapshotsDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  ChnHealthSnapshotsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// ChnHealthSnapshotsColumns defines and stores column names for the table chn_health_snapshots.
type ChnHealthSnapshotsColumns struct {
	Id                  string // 主键ID
	ChannelId           string // 关联渠道ID
	HealthScore         string // 综合健康度（0-100）
	SuccessRate         string // 请求成功率（0-100）
	LatencyMs           string // 平均延迟（毫秒）
	StabilityScore      string // 稳定性评分（0-100）
	ConsecutiveFailures string // 连续失败次数
	SnapshotAt          string // 快照时间
}

// chnHealthSnapshotsColumns holds the columns for the table chn_health_snapshots.
var chnHealthSnapshotsColumns = ChnHealthSnapshotsColumns{
	Id:                  "id",
	ChannelId:           "channel_id",
	HealthScore:         "health_score",
	SuccessRate:         "success_rate",
	LatencyMs:           "latency_ms",
	StabilityScore:      "stability_score",
	ConsecutiveFailures: "consecutive_failures",
	SnapshotAt:          "snapshot_at",
}

// NewChnHealthSnapshotsDao creates and returns a new DAO object for table data access.
func NewChnHealthSnapshotsDao(handlers ...gdb.ModelHandler) *ChnHealthSnapshotsDao {
	return &ChnHealthSnapshotsDao{
		group:    "default",
		table:    "chn_health_snapshots",
		columns:  chnHealthSnapshotsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChnHealthSnapshotsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChnHealthSnapshotsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChnHealthSnapshotsDao) Columns() ChnHealthSnapshotsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChnHealthSnapshotsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChnHealthSnapshotsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChnHealthSnapshotsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
