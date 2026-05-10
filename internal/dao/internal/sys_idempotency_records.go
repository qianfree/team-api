// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysIdempotencyRecordsDao is the data access object for the table sys_idempotency_records.
type SysIdempotencyRecordsDao struct {
	table    string                       // table is the underlying table name of the DAO.
	group    string                       // group is the database configuration group name of the current DAO.
	columns  SysIdempotencyRecordsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler           // handlers for customized model modification.
}

// SysIdempotencyRecordsColumns defines and stores column names for the table sys_idempotency_records.
type SysIdempotencyRecordsColumns struct {
	Id             string // 主键ID
	IdempotencyKey string // 幂等键（来自请求头 Idempotency-Key）
	RequestHash    string // 请求体哈希（SHA-256，用于校验请求一致性）
	ResponseBody   string // 首次处理的响应体（幂等返回时复用）
	Status         string // 状态：processing（处理中）/ completed（已完成）/ failed（失败）
	ExpiresAt      string // 过期时间（过期后记录可清理）
	CreatedAt      string // 创建时间
	UpdatedAt      string // 更新时间
}

// sysIdempotencyRecordsColumns holds the columns for the table sys_idempotency_records.
var sysIdempotencyRecordsColumns = SysIdempotencyRecordsColumns{
	Id:             "id",
	IdempotencyKey: "idempotency_key",
	RequestHash:    "request_hash",
	ResponseBody:   "response_body",
	Status:         "status",
	ExpiresAt:      "expires_at",
	CreatedAt:      "created_at",
	UpdatedAt:      "updated_at",
}

// NewSysIdempotencyRecordsDao creates and returns a new DAO object for table data access.
func NewSysIdempotencyRecordsDao(handlers ...gdb.ModelHandler) *SysIdempotencyRecordsDao {
	return &SysIdempotencyRecordsDao{
		group:    "default",
		table:    "sys_idempotency_records",
		columns:  sysIdempotencyRecordsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysIdempotencyRecordsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysIdempotencyRecordsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysIdempotencyRecordsDao) Columns() SysIdempotencyRecordsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysIdempotencyRecordsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysIdempotencyRecordsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysIdempotencyRecordsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
