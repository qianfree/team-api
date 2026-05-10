// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntOauthIdentitiesDao is the data access object for the table tnt_oauth_identities.
type TntOauthIdentitiesDao struct {
	table    string                    // table is the underlying table name of the DAO.
	group    string                    // group is the database configuration group name of the current DAO.
	columns  TntOauthIdentitiesColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler        // handlers for customized model modification.
}

// TntOauthIdentitiesColumns defines and stores column names for the table tnt_oauth_identities.
type TntOauthIdentitiesColumns struct {
	Id               string // 主键ID
	TenantId         string // 所属租户ID
	UserId           string // 关联的用户ID
	Provider         string // OAuth 供应商：github / google
	ProviderUserId   string // 供应商用户ID
	ProviderUsername string // 供应商用户名
	Email            string // 供应商返回的邮箱
	AvatarUrl        string // 供应商返回的头像URL
	AccessToken      string // 加密存储的 access_token
	RefreshToken     string // 加密存储的 refresh_token
	TokenExpiresAt   string // Token 过期时间
	RawData          string // 供应商原始返回数据
	CreatedAt        string //
	UpdatedAt        string //
}

// tntOauthIdentitiesColumns holds the columns for the table tnt_oauth_identities.
var tntOauthIdentitiesColumns = TntOauthIdentitiesColumns{
	Id:               "id",
	TenantId:         "tenant_id",
	UserId:           "user_id",
	Provider:         "provider",
	ProviderUserId:   "provider_user_id",
	ProviderUsername: "provider_username",
	Email:            "email",
	AvatarUrl:        "avatar_url",
	AccessToken:      "access_token",
	RefreshToken:     "refresh_token",
	TokenExpiresAt:   "token_expires_at",
	RawData:          "raw_data",
	CreatedAt:        "created_at",
	UpdatedAt:        "updated_at",
}

// NewTntOauthIdentitiesDao creates and returns a new DAO object for table data access.
func NewTntOauthIdentitiesDao(handlers ...gdb.ModelHandler) *TntOauthIdentitiesDao {
	return &TntOauthIdentitiesDao{
		group:    "default",
		table:    "tnt_oauth_identities",
		columns:  tntOauthIdentitiesColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntOauthIdentitiesDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntOauthIdentitiesDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntOauthIdentitiesDao) Columns() TntOauthIdentitiesColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntOauthIdentitiesDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntOauthIdentitiesDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntOauthIdentitiesDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
