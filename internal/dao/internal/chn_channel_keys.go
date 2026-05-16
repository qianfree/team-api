// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChnChannelKeysDao is the data access object for the table chn_channel_keys.
type ChnChannelKeysDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  ChnChannelKeysColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// ChnChannelKeysColumns defines and stores column names for the table chn_channel_keys.
type ChnChannelKeysColumns struct {
	Id             string // 主键ID
	ChannelId      string // 关联渠道ID
	Name           string // Key 别名（用于管理标识，如"主力Key"、"备用Key"）
	EncryptedKey   string // 加密存储的 API Key 原值（AES-256）
	Status         string // 状态：active（可用）/ disabled（禁用）/ exhausted（额度耗尽）
	LastUsedAt     string // 最后使用时间
	LastError      string // 最后一次错误信息
	CreatedAt      string // 创建时间
	KeyType        string // Key 类型：apikey（传统静态密钥）/ oauth（OAuth 令牌）
	TokenExpiresAt string // OAuth access_token 过期时间（仅 key_type=oauth 时有值）
	UpdatedAt      string // 更新时间
}

// chnChannelKeysColumns holds the columns for the table chn_channel_keys.
var chnChannelKeysColumns = ChnChannelKeysColumns{
	Id:             "id",
	ChannelId:      "channel_id",
	Name:           "name",
	EncryptedKey:   "encrypted_key",
	Status:         "status",
	LastUsedAt:     "last_used_at",
	LastError:      "last_error",
	CreatedAt:      "created_at",
	KeyType:        "key_type",
	TokenExpiresAt: "token_expires_at",
	UpdatedAt:      "updated_at",
}

// NewChnChannelKeysDao creates and returns a new DAO object for table data access.
func NewChnChannelKeysDao(handlers ...gdb.ModelHandler) *ChnChannelKeysDao {
	return &ChnChannelKeysDao{
		group:    "default",
		table:    "chn_channel_keys",
		columns:  chnChannelKeysColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChnChannelKeysDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChnChannelKeysDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChnChannelKeysDao) Columns() ChnChannelKeysColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChnChannelKeysDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChnChannelKeysDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChnChannelKeysDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
