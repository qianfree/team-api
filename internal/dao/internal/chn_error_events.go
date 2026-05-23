// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChnErrorEventsDao is the data access object for the table chn_error_events.
type ChnErrorEventsDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  ChnErrorEventsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// ChnErrorEventsColumns defines and stores column names for the table chn_error_events.
type ChnErrorEventsColumns struct {
	Id            string // 主键ID
	ChannelId     string // 发生错误的渠道ID
	ChannelName   string // 渠道名称（冗余存储，避免查询时JOIN）
	ChannelType   string // 渠道类型（ProviderType枚举值）
	Provider      string // 供应商名称（如 OpenAI, Claude, Zhipu 等）
	ModelName     string // 请求的模型名
	UpstreamModel string // 上游实际模型名（模型映射后）
	RequestId     string // 关联的请求唯一ID
	TenantId      string // 租户ID
	ErrorCategory string // 错误分类：rate_limit/auth_error/timeout/upstream_error/server_error/network_error/unknown
	StatusCode    string // HTTP状态码（来自上游响应或RelayError.StatusCode）
	ErrorType     string // RelayError.Type原始值（upstream_error/channel_error/auth_error等）
	ErrorMessage  string // 错误详细信息
	IsRetryable   string // 是否为可重试错误（429,500,502,503,504）
	Attempt       string // 重试轮次编号（0=首次）
	IsFinal       string // 是否为最终错误（非中间重试失败）
	LatencyMs     string // 请求耗时（毫秒）
	CreatedAt     string // 错误发生时间
}

// chnErrorEventsColumns holds the columns for the table chn_error_events.
var chnErrorEventsColumns = ChnErrorEventsColumns{
	Id:            "id",
	ChannelId:     "channel_id",
	ChannelName:   "channel_name",
	ChannelType:   "channel_type",
	Provider:      "provider",
	ModelName:     "model_name",
	UpstreamModel: "upstream_model",
	RequestId:     "request_id",
	TenantId:      "tenant_id",
	ErrorCategory: "error_category",
	StatusCode:    "status_code",
	ErrorType:     "error_type",
	ErrorMessage:  "error_message",
	IsRetryable:   "is_retryable",
	Attempt:       "attempt",
	IsFinal:       "is_final",
	LatencyMs:     "latency_ms",
	CreatedAt:     "created_at",
}

// NewChnErrorEventsDao creates and returns a new DAO object for table data access.
func NewChnErrorEventsDao(handlers ...gdb.ModelHandler) *ChnErrorEventsDao {
	return &ChnErrorEventsDao{
		group:    "default",
		table:    "chn_error_events",
		columns:  chnErrorEventsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChnErrorEventsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChnErrorEventsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChnErrorEventsDao) Columns() ChnErrorEventsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChnErrorEventsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChnErrorEventsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChnErrorEventsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
