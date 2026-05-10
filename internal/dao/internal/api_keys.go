// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ApiKeysDao is the data access object for the table api_keys.
type ApiKeysDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ApiKeysColumns     // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ApiKeysColumns defines and stores column names for the table api_keys.
type ApiKeysColumns struct {
	Id                   string // 主键ID
	TenantId             string // 所属租户ID
	UserId               string // 创建者用户ID
	Name                 string // Key 名称（如 "生产环境"、"测试用"）
	EncryptedKey         string // 加密存储的完整 API Key（AES-256）
	KeyPrefix            string // Key 前缀（用于快速查找，明文存储，如 sk-a1b2c3d4）
	Scope                string // 权限范围：full（全部）/ chat_only（仅对话）/ embeddings_only（仅嵌入）/ images_only（仅图像）/ read_only（只读）/ custom（自定义）
	Status               string // 状态：active（正常）/ disabled（禁用）/ expired（已过期）
	ExpiresAt            string // 过期时间（NULL 表示永不过期）
	RateLimitQps         string // QPS 限流阈值（NULL 表示使用默认值）
	RateLimitConcurrency string // 并发限制阈值（NULL 表示使用默认值）
	IpWhitelist          string // IP 白名单数组（NULL 或空数组表示不限制）
	TotalQuota           string // 额度上限（NULL 表示不限制）
	UsedQuota            string // 已使用额度
	ProjectId            string // 关联项目ID（NULL 表示不属于任何项目）
	CreatedAt            string // 创建时间
	UpdatedAt            string // 更新时间
	KeyType              string // 密钥类型：personal（个人密钥）/ project（项目密钥）
}

// apiKeysColumns holds the columns for the table api_keys.
var apiKeysColumns = ApiKeysColumns{
	Id:                   "id",
	TenantId:             "tenant_id",
	UserId:               "user_id",
	Name:                 "name",
	EncryptedKey:         "encrypted_key",
	KeyPrefix:            "key_prefix",
	Scope:                "scope",
	Status:               "status",
	ExpiresAt:            "expires_at",
	RateLimitQps:         "rate_limit_qps",
	RateLimitConcurrency: "rate_limit_concurrency",
	IpWhitelist:          "ip_whitelist",
	TotalQuota:           "total_quota",
	UsedQuota:            "used_quota",
	ProjectId:            "project_id",
	CreatedAt:            "created_at",
	UpdatedAt:            "updated_at",
	KeyType:              "key_type",
}

// NewApiKeysDao creates and returns a new DAO object for table data access.
func NewApiKeysDao(handlers ...gdb.ModelHandler) *ApiKeysDao {
	return &ApiKeysDao{
		group:    "default",
		table:    "api_keys",
		columns:  apiKeysColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ApiKeysDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ApiKeysDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ApiKeysDao) Columns() ApiKeysColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ApiKeysDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ApiKeysDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ApiKeysDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
