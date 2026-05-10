// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChnHealthScoresDao is the data access object for the table chn_health_scores.
type ChnHealthScoresDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  ChnHealthScoresColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// ChnHealthScoresColumns defines and stores column names for the table chn_health_scores.
type ChnHealthScoresColumns struct {
	Id                  string // 主键ID
	ChannelId           string // 关联渠道ID
	SuccessRate         string // 请求成功率（0-100）
	LatencyMs           string // 平均延迟（毫秒）
	StabilityScore      string // 稳定性评分（0-100，基于延迟波动计算）
	ConsecutiveFailures string // 连续失败次数（成功后归零）
	HealthScore         string // 综合健康度（0-100）= 成功率×0.40 + 延迟分×0.25 + 稳定性×0.20 + 连续失败分×0.15
	CalculatedAt        string // 最近一次计算时间
	CreatedAt           string // 创建时间
	UpdatedAt           string // 更新时间
}

// chnHealthScoresColumns holds the columns for the table chn_health_scores.
var chnHealthScoresColumns = ChnHealthScoresColumns{
	Id:                  "id",
	ChannelId:           "channel_id",
	SuccessRate:         "success_rate",
	LatencyMs:           "latency_ms",
	StabilityScore:      "stability_score",
	ConsecutiveFailures: "consecutive_failures",
	HealthScore:         "health_score",
	CalculatedAt:        "calculated_at",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
}

// NewChnHealthScoresDao creates and returns a new DAO object for table data access.
func NewChnHealthScoresDao(handlers ...gdb.ModelHandler) *ChnHealthScoresDao {
	return &ChnHealthScoresDao{
		group:    "default",
		table:    "chn_health_scores",
		columns:  chnHealthScoresColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChnHealthScoresDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChnHealthScoresDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChnHealthScoresDao) Columns() ChnHealthScoresColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChnHealthScoresDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChnHealthScoresDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChnHealthScoresDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
