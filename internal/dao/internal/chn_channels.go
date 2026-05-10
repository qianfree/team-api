// ==========================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// ==========================================================================

package internal

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ChnChannelsDao is the data access object for the table chn_channels.
type ChnChannelsDao struct {
	table    string             // table is the underlying table name of the DAO.
	group    string             // group is the database configuration group name of the current DAO.
	columns  ChnChannelsColumns // columns contains all the column names of Table for convenient usage.
	handlers []gdb.ModelHandler // handlers for customized model modification.
}

// ChnChannelsColumns defines and stores column names for the table chn_channels.
type ChnChannelsColumns struct {
	Id                       string // 主键ID
	Name                     string // 渠道显示名称（如 "OpenAI 主力"、"Claude 备用"）
	Type                     string // 供应商类型：1=OpenAI, 2=Anthropic Claude, 3=Google Gemini, 4=阿里云百炼, 5=百度文心, 6=腾讯混元, 7=智谱AI, 8=DeepSeek, 9=Moonshot, 10=火山引擎, 11=AWS Bedrock, 12=Azure OpenAI, 13=Google Vertex AI, 14=Cohere, 15=Mistral, 16=xAI
	BaseUrl                  string // API 基础地址
	Status                   string // 状态：active（启用）/ disabled（禁用）/ testing（测试中）
	Priority                 string // 优先级（数字越大越优先，调度时优先选择高优先级渠道）
	Weight                   string // 权重（同优先级下按权重随机选择，范围 1-100）
	MaxConcurrency           string // 最大并发请求数
	Settings                 string // 渠道配置（JSONB）：超时时间、重试次数等
	TestModel                string // 测试使用的模型名
	Remark                   string // 备注
	CreatedBy                string // 创建者管理员ID
	CreatedAt                string // 创建时间
	UpdatedAt                string // 更新时间
	IsVip                    string // 是否VIP专属渠道
	SharingThreshold         string // 允许普通租户借用的利用率阈值（如0.6表示利用率<60%时可借用）
	PreemptionThreshold      string // 触发VIP抢占的利用率阈值（如0.8表示利用率>=80%时VIP可抢占）
	BorrowingCooldownSeconds string // 普通租户被抢占后的冷却时间（秒）
	AutoDisabled             string // 是否被自动禁用：0=否, 1=是（由连续失败触发）
}

// chnChannelsColumns holds the columns for the table chn_channels.
var chnChannelsColumns = ChnChannelsColumns{
	Id:                       "id",
	Name:                     "name",
	Type:                     "type",
	BaseUrl:                  "base_url",
	Status:                   "status",
	Priority:                 "priority",
	Weight:                   "weight",
	MaxConcurrency:           "max_concurrency",
	Settings:                 "settings",
	TestModel:                "test_model",
	Remark:                   "remark",
	CreatedBy:                "created_by",
	CreatedAt:                "created_at",
	UpdatedAt:                "updated_at",
	IsVip:                    "is_vip",
	SharingThreshold:         "sharing_threshold",
	PreemptionThreshold:      "preemption_threshold",
	BorrowingCooldownSeconds: "borrowing_cooldown_seconds",
	AutoDisabled:             "auto_disabled",
}

// NewChnChannelsDao creates and returns a new DAO object for table data access.
func NewChnChannelsDao(handlers ...gdb.ModelHandler) *ChnChannelsDao {
	return &ChnChannelsDao{
		group:    "default",
		table:    "chn_channels",
		columns:  chnChannelsColumns,
		handlers: handlers,
	}
}

// DB retrieves and returns the underlying raw database management object of the current DAO.
func (dao *ChnChannelsDao) DB() gdb.DB {
	return g.DB(dao.group)
}

// Table returns the table name of the current DAO.
func (dao *ChnChannelsDao) Table() string {
	return dao.table
}

// Columns returns all column names of the current DAO.
func (dao *ChnChannelsDao) Columns() ChnChannelsColumns {
	return dao.columns
}

// Group returns the database configuration group name of the current DAO.
func (dao *ChnChannelsDao) Group() string {
	return dao.group
}

// Ctx creates and returns a Model for the current DAO. It automatically sets the context for the current operation.
func (dao *ChnChannelsDao) Ctx(ctx context.Context) *gdb.Model {
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
func (dao *ChnChannelsDao) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) (err error) {
	return dao.Ctx(ctx).Transaction(ctx, f)
}
