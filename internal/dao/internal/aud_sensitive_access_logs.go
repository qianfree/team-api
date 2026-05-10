// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AudSensitiveAccessLogsDao is the data access object for the table aud_sensitive_access_logs.
type AudSensitiveAccessLogsDao struct {
	table    string                        // table is the underlying table name of the DAO.
	group    string                        // group is the database configuration group name of the current DAO.
	columns  AudSensitiveAccessLogsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler            // handlers for customized model modification.
}

// AudSensitiveAccessLogsColumns defines and stores column names for the table aud_sensitive_access_logs.
type AudSensitiveAccessLogsColumns struct {
	Id           string // 主键ID
	UserId       string // 访问者用户ID
	UserType     string // 访问者类型：admin（管理后台）/ tenant（租户控制台）
	ResourceType string // 资源类型（如 api_key、channel、wallet 等）
	ResourceId   string // 资源ID
	Action       string // 访问动作（如 view、export、download）
	Reason       string // 访问原因（查看敏感数据时需填写）
	IpAddress    string // 访问者 IP 地址
	UserAgent    string // 访问者 User-Agent
	CreatedAt    string // 创建时间
}

// audSensitiveAccessLogsColumns holds the columns for the table aud_sensitive_access_logs.
var audSensitiveAccessLogsColumns = AudSensitiveAccessLogsColumns{
	Id:           "id",
	UserId:       "user_id",
	UserType:     "user_type",
	ResourceType: "resource_type",
	ResourceId:   "resource_id",
	Action:       "action",
	Reason:       "reason",
	IpAddress:    "ip_address",
	UserAgent:    "user_agent",
	CreatedAt:    "created_at",
}

// NewAudSensitiveAccessLogsDao creates and returns a new DAO object for table data access.
func NewAudSensitiveAccessLogsDao(handlers ...gdb.ModelHandler) *AudSensitiveAccessLogsDao {
	return &AudSensitiveAccessLogsDao{
		group:    "default",
		table:    "aud_sensitive_access_logs",
		columns:  audSensitiveAccessLogsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AudSensitiveAccessLogsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AudSensitiveAccessLogsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AudSensitiveAccessLogsDao) Columns() AudSensitiveAccessLogsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AudSensitiveAccessLogsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AudSensitiveAccessLogsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AudSensitiveAccessLogsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
