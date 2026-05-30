// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// SysSessionsDao is the data access object for the table sys_sessions.
type SysSessionsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  SysSessionsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// SysSessionsColumns defines and stores column names for the table sys_sessions.
type SysSessionsColumns struct {
	Id               string // 主键ID
	UserType         string // 用户类型：admin（管理后台）/ tenant（租户控制台）
	UserId           string // 用户ID
	TenantId         string // 租户ID（admin类型时为NULL）
	RefreshTokenHash string // Refresh Token 哈希值
	DeviceInfo       string // 设备信息（JSONB：浏览器、操作系统等）
	IpAddress        string // 登录IP地址
	ExpiresAt        string // Token 过期时间
	CreatedAt        string // 创建时间
	UpdatedAt        string // 更新时间
	Jti              string // JWT ID (jti)，会话唯一标识符（UUID），用于 Redis 吊销缓存
}

// sysSessionsColumns holds the columns for the table sys_sessions.
var sysSessionsColumns = SysSessionsColumns{
	Id:               "id",
	UserType:         "user_type",
	UserId:           "user_id",
	TenantId:         "tenant_id",
	RefreshTokenHash: "refresh_token_hash",
	DeviceInfo:       "device_info",
	IpAddress:        "ip_address",
	ExpiresAt:        "expires_at",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
	Jti:              "jti",
}

// NewSysSessionsDao creates and returns a new DAO object for table data access.
func NewSysSessionsDao(handlers ...gdb.ModelHandler) *SysSessionsDao {
	return &SysSessionsDao{
		group:    "default",
		table:    "sys_sessions",
		columns:  sysSessionsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *SysSessionsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *SysSessionsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *SysSessionsDao) Columns() SysSessionsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *SysSessionsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *SysSessionsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *SysSessionsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
