// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TntTenantsDao is the data access object for the table tnt_tenants.
type TntTenantsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  TntTenantsColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// TntTenantsColumns defines and stores column names for the table tnt_tenants.
type TntTenantsColumns struct {
	Id                  string // 主键ID
	Name                string // 租户显示名称（如公司名）
	Code                string // 租户代码（唯一标识，用于 RAM 账号格式 username@tenant_code）
	LogoUrl             string // 租户 Logo URL
	OwnerUserId         string // 所有者用户ID（关联 tnt_users.id）
	Status              string // 状态：trial（试用）/ active（活跃）/ past_due（逾期）/ frozen（冻结）/ terminated（已终止）/ free（免费版）/ suspended（暂停）/ closed（关闭）
	MaxMembers          string // 最大成员数上限
	Settings            string // 租户配置（JSONB）：通知偏好、安全策略、IP 白名单等
	CreatedAt           string // 创建时间
	UpdatedAt           string // 更新时间
	TrialEndsAt         string // 试用期结束时间
	GracePeriodEndsAt   string // 宽限期结束时间（套餐到期后 7 天）
	FrozenAt            string // 冻结时间
	ClosingRequestedAt  string // 主动申请注销时间（7 天冷静期）
	DataRemovalAt       string // 数据清除时间（冻结 30 天后）
	MaxConcurrency      string // 租户总并发上限（0表示不限制）
	DefaultChannelScope string // 默认渠道范围（NULL或[]表示全部可用，否则为channel_id数组）
}

// tntTenantsColumns holds the columns for the table tnt_tenants.
var tntTenantsColumns = TntTenantsColumns{
	Id:                  "id",
	Name:                "name",
	Code:                "code",
	LogoUrl:             "logo_url",
	OwnerUserId:         "owner_user_id",
	Status:              "status",
	MaxMembers:          "max_members",
	Settings:            "settings",
	CreatedAt:           "created_at",
	UpdatedAt:           "updated_at",
	TrialEndsAt:         "trial_ends_at",
	GracePeriodEndsAt:   "grace_period_ends_at",
	FrozenAt:            "frozen_at",
	ClosingRequestedAt:  "closing_requested_at",
	DataRemovalAt:       "data_removal_at",
	MaxConcurrency:      "max_concurrency",
	DefaultChannelScope: "default_channel_scope",
}

// NewTntTenantsDao creates and returns a new DAO object for table data access.
func NewTntTenantsDao(handlers ...gdb.ModelHandler) *TntTenantsDao {
	return &TntTenantsDao{
		group:    "default",
		table:    "tnt_tenants",
		columns:  tntTenantsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *TntTenantsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *TntTenantsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *TntTenantsDao) Columns() TntTenantsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *TntTenantsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *TntTenantsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *TntTenantsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
