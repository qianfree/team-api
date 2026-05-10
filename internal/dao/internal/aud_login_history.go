// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// AudLoginHistoryDao is the data access object for the table aud_login_history.
type AudLoginHistoryDao struct {
	table    string                 // table is the underlying table name of the DAO.
	group    string                 // group is the database configuration group name of the current DAO.
	columns  AudLoginHistoryColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler     // handlers for customized model modification.
}

// AudLoginHistoryColumns defines and stores column names for the table aud_login_history.
type AudLoginHistoryColumns struct {
	Id                string // 主键ID
	UserType          string // 用户类型：admin（管理后台）/ tenant（租户控制台）
	UserId            string // 用户ID
	TenantId          string // 租户ID（仅 tenant 类型用户有值）
	LoginMethod       string // 登录方式：password（密码）/ totp（双因素）/ sso（单点登录）/ backup_code（恢复码）
	IpAddress         string // 登录IP地址
	UserAgent         string // 浏览器 User-Agent
	DeviceFingerprint string // 设备指纹（用于检测新设备登录）
	Location          string // IP 地理位置
	IsNewDevice       string // 是否为新设备
	Success           string // 登录是否成功
	FailReason        string // 登录失败原因
	CreatedAt         string // 登录时间
}

// audLoginHistoryColumns holds the columns for the table aud_login_history.
var audLoginHistoryColumns = AudLoginHistoryColumns{
	Id:                "id",
	UserType:          "user_type",
	UserId:            "user_id",
	TenantId:          "tenant_id",
	LoginMethod:       "login_method",
	IpAddress:         "ip_address",
	UserAgent:         "user_agent",
	DeviceFingerprint: "device_fingerprint",
	Location:          "location",
	IsNewDevice:       "is_new_device",
	Success:           "success",
	FailReason:        "fail_reason",
	CreatedAt:         "created_at",
}

// NewAudLoginHistoryDao creates and returns a new DAO object for table data access.
func NewAudLoginHistoryDao(handlers ...gdb.ModelHandler) *AudLoginHistoryDao {
	return &AudLoginHistoryDao{
		group:    "default",
		table:    "aud_login_history",
		columns:  audLoginHistoryColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *AudLoginHistoryDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *AudLoginHistoryDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *AudLoginHistoryDao) Columns() AudLoginHistoryColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *AudLoginHistoryDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *AudLoginHistoryDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *AudLoginHistoryDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
