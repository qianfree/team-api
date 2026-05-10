// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlPricing is the golang structure for table mdl_pricing.
type MdlPricing struct {
	Id                 int64       `json:"id"                   orm:"id"                   description:""`                                              //
	ModelId            int64       `json:"model_id"             orm:"model_id"             description:"关联模型ID"`                                        // 关联模型ID
	BillingMode        string      `json:"billing_mode"         orm:"billing_mode"         description:"计费模式：token（按量）/ per_request（按次）/ tiered（阶梯按量）"` // 计费模式：token（按量）/ per_request（按次）/ tiered（阶梯按量）
	MinTokens          int64       `json:"min_tokens"           orm:"min_tokens"           description:"阶梯起始 token 数（仅 tiered 模式，其他模式为 0）"`             // 阶梯起始 token 数（仅 tiered 模式，其他模式为 0）
	MaxTokens          int64       `json:"max_tokens"           orm:"max_tokens"           description:"阶梯结束 token 数（NULL=无上限，仅 tiered 模式）"`            // 阶梯结束 token 数（NULL=无上限，仅 tiered 模式）
	InputPrice         float64     `json:"input_price"          orm:"input_price"          description:"每 1M input token 价格（token/tiered 模式）"`          // 每 1M input token 价格（token/tiered 模式）
	OutputPrice        float64     `json:"output_price"         orm:"output_price"         description:"每 1M output token 价格（token/tiered 模式）"`         // 每 1M output token 价格（token/tiered 模式）
	PerRequestPrice    float64     `json:"per_request_price"    orm:"per_request_price"    description:"按次计费单价（仅 per_request 模式）"`                      // 按次计费单价（仅 per_request 模式）
	CreatedAt          *gtime.Time `json:"created_at"           orm:"created_at"           description:""`                                              //
	UpdatedAt          *gtime.Time `json:"updated_at"           orm:"updated_at"           description:""`                                              //
	CacheReadPrice     float64     `json:"cache_read_price"     orm:"cache_read_price"     description:"缓存读取每 1M token 价格（直接定价）"`                       // 缓存读取每 1M token 价格（直接定价）
	CacheCreationPrice float64     `json:"cache_creation_price" orm:"cache_creation_price" description:"缓存创建每 1M token 价格（直接定价）"`                       // 缓存创建每 1M token 价格（直接定价）
}
