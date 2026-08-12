// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilUsageDailyDao is the data access object for the table bil_usage_daily.
type BilUsageDailyDao struct {
	table    string               // table is the underlying table name of the DAO.
	group    string               // group is the database configuration group name of the current DAO.
	columns  BilUsageDailyColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler   // handlers for customized model modification.
}

// BilUsageDailyColumns defines and stores column names for the table bil_usage_daily.
type BilUsageDailyColumns struct {
	Id              string //
	StatDate        string // 统计日期（按 created_at 的日期分桶）
	TenantId        string // 租户ID
	ProjectId       string // 项目ID（0=无项目/个人Key）
	ModelName       string // 模型名称
	ChannelId       string // 渠道ID（0=无渠道）
	Status          string // 请求状态：success/error/timeout/cancelled
	RequestCount    string // 请求数（COUNT(*)）
	InputTokens     string // 输入Token合计
	OutputTokens    string // 输出Token合计
	TotalCost       string // 客户侧成本合计（USD，源自 bil_usage_logs.total_cost）
	AccountCost     string // 上游账户成本合计（USD，源自 bil_usage_logs.account_cost，用于利润分析）
	SumLatencyMs    string // 总延迟合计（ms，源自 bil_usage_logs.latency_ms；视图按 SUM/COUNT 求均值）
	SumFirstTokenMs string // 首Token延迟合计（ms，源自 bil_usage_logs.first_token_ms；视图按 SUM/COUNT 求均值）
	CreatedAt       string //
	UpdatedAt       string //
}

// bilUsageDailyColumns holds the columns for the table bil_usage_daily.
var bilUsageDailyColumns = BilUsageDailyColumns{
	Id:              "id",
	StatDate:        "stat_date",
	TenantId:        "tenant_id",
	ProjectId:       "project_id",
	ModelName:       "model_name",
	ChannelId:       "channel_id",
	Status:          "status",
	RequestCount:    "request_count",
	InputTokens:     "input_tokens",
	OutputTokens:    "output_tokens",
	TotalCost:       "total_cost",
	AccountCost:     "account_cost",
	SumLatencyMs:    "sum_latency_ms",
	SumFirstTokenMs: "sum_first_token_ms",
	CreatedAt:       "created_at",
	UpdatedAt:       "updated_at",
}

// NewBilUsageDailyDao creates and returns a new DAO object for table data access.
func NewBilUsageDailyDao(handlers ...gdb.ModelHandler) *BilUsageDailyDao {
	return &BilUsageDailyDao{
		group:    "default",
		table:    "bil_usage_daily",
		columns:  bilUsageDailyColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilUsageDailyDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilUsageDailyDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilUsageDailyDao) Columns() BilUsageDailyColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilUsageDailyDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilUsageDailyDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilUsageDailyDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
