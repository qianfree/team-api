// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// OpnAppsDao is the data access object for the table opn_apps.
type OpnAppsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  OpnAppsColumns     // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// OpnAppsColumns defines and stores column names for the table opn_apps.
type OpnAppsColumns struct {
	Id            string // 主键ID
	TenantId      string // 所属租户ID
	Name          string // 应用名称
	Description   string // 应用描述
	AppId         string // 应用标识（opn_xxx 格式）
	AppSecretHash string // App Secret 哈希（bcrypt）
	Permissions   string // 权限范围（JSON 数组）
	IpWhitelist   string // IP 白名单（JSON 数组，为空则不限制）
	CallbackUrl   string // OAuth 回调 URL
	IsSandbox     string // 是否沙箱应用
	Status        string // 状态：active（启用）/ disabled（禁用）
	RateLimit     string // 每分钟请求上限
	LastUsedAt    string // 最后使用时间
	CreatedAt     string // 创建时间
	UpdatedAt     string // 更新时间
}

// opnAppsColumns holds the columns for the table opn_apps.
var opnAppsColumns = OpnAppsColumns{
	Id:            "id",
	TenantId:      "tenant_id",
	Name:          "name",
	Description:   "description",
	AppId:         "app_id",
	AppSecretHash: "app_secret_hash",
	Permissions:   "permissions",
	IpWhitelist:   "ip_whitelist",
	CallbackUrl:   "callback_url",
	IsSandbox:     "is_sandbox",
	Status:        "status",
	RateLimit:     "rate_limit",
	LastUsedAt:    "last_used_at",
	CreatedAt:     "created_at",
	UpdatedAt:     "updated_at",
}

// NewOpnAppsDao creates and returns a new DAO object for table data access.
func NewOpnAppsDao(handlers ...gdb.ModelHandler) *OpnAppsDao {
	return &OpnAppsDao{
		group:    "default",
		table:    "opn_apps",
		columns:  opnAppsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *OpnAppsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *OpnAppsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *OpnAppsDao) Columns() OpnAppsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *OpnAppsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *OpnAppsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *OpnAppsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
