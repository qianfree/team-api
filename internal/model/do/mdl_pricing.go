// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlPricing is the golang structure of table mdl_pricing for DAO operations like Where/Data.
type MdlPricing struct {
	g.Meta             `orm:"table:mdl_pricing, do:true"`
	Id                 any         //
	ModelId            any         // 关联模型ID
	BillingMode        any         // 计费模式：token（按量）/ per_request（按次）/ tiered（阶梯按量）
	MinTokens          any         // 阶梯起始 token 数（仅 tiered 模式，其他模式为 0）
	MaxTokens          any         // 阶梯结束 token 数（NULL=无上限，仅 tiered 模式）
	InputPrice         any         // 每 1M input token 价格（token/tiered 模式）
	OutputPrice        any         // 每 1M output token 价格（token/tiered 模式）
	PerRequestPrice    any         // 按次计费单价（仅 per_request 模式）
	CreatedAt          *gtime.Time //
	UpdatedAt          *gtime.Time //
	CacheReadPrice     any         // 缓存读取每 1M token 价格（直接定价）
	CacheCreationPrice any         // 缓存创建每 1M token 价格（直接定价）
}
