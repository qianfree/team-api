// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// BilRecordsDao is the data access object for the table bil_records.
type BilRecordsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  BilRecordsColumns  // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// BilRecordsColumns defines and stores column names for the table bil_records.
type BilRecordsColumns struct {
	Id                      string // 主键ID
	TenantId                string // 租户ID
	UserId                  string // 用户ID
	ApiKeyId                string // 使用的 API Key ID
	ChannelId               string // 使用的渠道ID
	ModelName               string // 调用的模型名
	RequestId               string // 请求唯一ID（关联全链路追踪）
	RelayMode               string // 代理模式：chat_completions / embeddings / images_generations 等
	InputTokens             string // 输入 token 数
	OutputTokens            string // 输出 token 数
	InputPrice              string // 计费时输入单价（快照，防止价格变更影响历史记录）
	OutputPrice             string // 计费时输出单价（快照）
	TotalCost               string // 最终费用 = 基础价格 × 模型倍率 × 租户倍率
	Currency                string // 货币（USD）
	Status                  string // 状态：pre_deducted（已预扣）/ settled（已结算）/ refunded（已退款）
	SettledAt               string // 结算时间
	CreatedAt               string // 创建时间
	UpdatedAt               string // 更新时间
	BillingMode             string // 实际计费模式
	EffectiveInputPrice     string // 实际生效的输入单价（快照）
	EffectiveOutputPrice    string // 实际生效的输出单价（快照）
	DiscountRatio           string // 实际折扣比例（快照）
	BillingInputMultiplier  string // 梯度定价输入乘数（快照）
	BillingOutputMultiplier string // 梯度定价输出乘数（快照）
	CacheCreationTokens     string // 缓存创建 token 数
	CacheReadTokens         string // 缓存读取 token 数
	CacheCreationCost       string // 缓存创建费用
	CacheReadCost           string // 缓存读取费用
	ModelMultiplier         string // 模型倍率（快照）
	TenantMultiplier        string // 租户倍率（快照）
	BaseInputPrice          string // 基础模型输入单价（快照，应用倍率前）
	BaseOutputPrice         string // 基础模型输出单价（快照，应用倍率前）
	BillingSnapshot         string // 完整计费计算过程快照（JSONB）
	TenantPlanId            string //
	PlanDeduction           string //
	WalletDeduction         string //
}

// bilRecordsColumns holds the columns for the table bil_records.
var bilRecordsColumns = BilRecordsColumns{
	Id:                      "id",
	TenantId:                "tenant_id",
	UserId:                  "user_id",
	ApiKeyId:                "api_key_id",
	ChannelId:               "channel_id",
	ModelName:               "model_name",
	RequestId:               "request_id",
	RelayMode:               "relay_mode",
	InputTokens:             "input_tokens",
	OutputTokens:            "output_tokens",
	InputPrice:              "input_price",
	OutputPrice:             "output_price",
	TotalCost:               "total_cost",
	Currency:                "currency",
	Status:                  "status",
	SettledAt:               "settled_at",
	CreatedAt:               "created_at",
	UpdatedAt:               "updated_at",
	BillingMode:             "billing_mode",
	EffectiveInputPrice:     "effective_input_price",
	EffectiveOutputPrice:    "effective_output_price",
	DiscountRatio:           "discount_ratio",
	BillingInputMultiplier:  "billing_input_multiplier",
	BillingOutputMultiplier: "billing_output_multiplier",
	CacheCreationTokens:     "cache_creation_tokens",
	CacheReadTokens:         "cache_read_tokens",
	CacheCreationCost:       "cache_creation_cost",
	CacheReadCost:           "cache_read_cost",
	ModelMultiplier:         "model_multiplier",
	TenantMultiplier:        "tenant_multiplier",
	BaseInputPrice:          "base_input_price",
	BaseOutputPrice:         "base_output_price",
	BillingSnapshot:         "billing_snapshot",
	TenantPlanId:            "tenant_plan_id",
	PlanDeduction:           "plan_deduction",
	WalletDeduction:         "wallet_deduction",
}

// NewBilRecordsDao creates and returns a new DAO object for table data access.
func NewBilRecordsDao(handlers ...gdb.ModelHandler) *BilRecordsDao {
	return &BilRecordsDao{
		group:    "default",
		table:    "bil_records",
		columns:  bilRecordsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *BilRecordsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *BilRecordsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *BilRecordsDao) Columns() BilRecordsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *BilRecordsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *BilRecordsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *BilRecordsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
