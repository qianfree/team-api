// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// PlnTenantPlansDao is the data access object for the table pln_tenant_plans.
type PlnTenantPlansDao struct {
	table    string                // table is the underlying table name of the DAO.
	group    string                // group is the database configuration group name of the current DAO.
	columns  PlnTenantPlansColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler    // handlers for customized model modification.
}

// PlnTenantPlansColumns defines and stores column names for the table pln_tenant_plans.
type PlnTenantPlansColumns struct {
	Id                 string // 主键ID
	TenantId           string // 租户ID
	PlanId             string // 套餐ID
	Status             string // 状态：pending（待生效）/ active（生效中）/ expired（已过期）/ cancelled（已取消）
	StartAt            string // 生效起始时间
	EndAt              string // 到期时间
	AutoRenew          string // 是否自动续费
	MonthlyQuotaTokens string // 月度 Token 配额快照
	UsedTokens         string // 本月已使用 Token
	LastResetAt        string // 上次配额重置时间
	CancelledAt        string // 取消时间
	CreatedAt          string // 创建时间
	UpdatedAt          string // 更新时间
}

// plnTenantPlansColumns holds the columns for the table pln_tenant_plans.
var plnTenantPlansColumns = PlnTenantPlansColumns{
	Id:                 "id",
	TenantId:           "tenant_id",
	PlanId:             "plan_id",
	Status:             "status",
	StartAt:            "start_at",
	EndAt:              "end_at",
	AutoRenew:          "auto_renew",
	MonthlyQuotaTokens: "monthly_quota_tokens",
	UsedTokens:         "used_tokens",
	LastResetAt:        "last_reset_at",
	CancelledAt:        "cancelled_at",
	CreatedAt:          "created_at",
	UpdatedAt:          "updated_at",
}

// NewPlnTenantPlansDao creates and returns a new DAO object for table data access.
func NewPlnTenantPlansDao(handlers ...gdb.ModelHandler) *PlnTenantPlansDao {
	return &PlnTenantPlansDao{
		group:    "default",
		table:    "pln_tenant_plans",
		columns:  plnTenantPlansColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *PlnTenantPlansDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *PlnTenantPlansDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *PlnTenantPlansDao) Columns() PlnTenantPlansColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *PlnTenantPlansDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *PlnTenantPlansDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *PlnTenantPlansDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
