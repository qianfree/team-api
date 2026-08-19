// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// MdlPricing is the golang structure of table mdl_pricing for DAO operations like Where/Data.
type MdlPricing struct {
	g.Meta             `orm:"table:mdl_pricing, do:true"`
	Id                 any              //
	ModelId            any              // 关联模型ID
	BillingMode        any              // 计费模式：token（按量）/ per_request（按次）/ tiered（阶梯按量）
	MinTokens          any              // 阶梯起始 token 数（仅 tiered 模式，其他模式为 0）
	MaxTokens          any              // 阶梯结束 token 数（NULL=无上限，仅 tiered 模式）
	InputPrice         any              // 每 1M input token 价格（token/tiered 模式）
	OutputPrice        any              // 每 1M output token 价格（token/tiered 模式）
	PerRequestPrice    *decimal.Decimal // 按次计费单价（仅 per_request 模式）
	CreatedAt          *gtime.Time      //
	UpdatedAt          *gtime.Time      //
	CacheReadPrice     any              // 缓存读取每 1M token 价格（直接定价）
	CacheCreationPrice any              // 缓存创建每 1M token 价格（直接定价）
	TimeSegments       any              // 时段定价（JSONB 有序数组，仅 min_tokens=0 锚点行生效）：[{"name":"闲时","days":[1,2,3,4,5],"start_time":"00:00","end_time":"08:00","valid_from":"","valid_to":"","multiplier":0.5}]，按序先命中先生效，未命中=默认价（乘数 1.0），days 1=周一..7=周日 空=每天，end<start 表示跨零点
}
