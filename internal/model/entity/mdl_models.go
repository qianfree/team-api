// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlModels is the golang structure for table mdl_models.
type MdlModels struct {
	Id               int64       `json:"id"                 orm:"id"                 description:"主键ID"`                                                            // 主键ID
	ModelId          string      `json:"model_id"           orm:"model_id"           description:"模型唯一标识（如 gpt-4o、claude-3-5-sonnet）"`                              // 模型唯一标识（如 gpt-4o、claude-3-5-sonnet）
	ModelName        string      `json:"model_name"         orm:"model_name"         description:"模型显示名称（如 GPT-4o、Claude 3.5 Sonnet）"`                              // 模型显示名称（如 GPT-4o、Claude 3.5 Sonnet）
	Category         string      `json:"category"           orm:"category"           description:"模型分类：chat（对话）/ embedding（嵌入）/ image（图像）/ audio（音频）/ rerank（重排序）"` // 模型分类：chat（对话）/ embedding（嵌入）/ image（图像）/ audio（音频）/ rerank（重排序）
	Status           string      `json:"status"             orm:"status"             description:"状态：active（可用）/ deprecated（已废弃）/ offline（已下线）"`                    // 状态：active（可用）/ deprecated（已废弃）/ offline（已下线）
	MaxContextTokens int         `json:"max_context_tokens" orm:"max_context_tokens" description:"最大上下文 token 数"`                                                   // 最大上下文 token 数
	MaxOutputTokens  int         `json:"max_output_tokens"  orm:"max_output_tokens"  description:"最大输出 token 数"`                                                    // 最大输出 token 数
	Description      string      `json:"description"        orm:"description"        description:"模型描述"`                                                            // 模型描述
	Tags             []string    `json:"tags"               orm:"tags"               description:"标签（如 reasoning、vision、function_calling）"`                         // 标签（如 reasoning、vision、function_calling）
	CreatedAt        *gtime.Time `json:"created_at"         orm:"created_at"         description:"创建时间"`                                                            // 创建时间
	UpdatedAt        *gtime.Time `json:"updated_at"         orm:"updated_at"         description:"更新时间"`                                                            // 更新时间
	DeprecatedAt     *gtime.Time `json:"deprecated_at"      orm:"deprecated_at"      description:"标记弃用的时间（NULL表示未弃用）"`                                              // 标记弃用的时间（NULL表示未弃用）
	SunsetDate       *gtime.Time `json:"sunset_date"        orm:"sunset_date"        description:"计划下线日期（到达后返回410 Gone，NULL表示未设置）"`                                 // 计划下线日期（到达后返回410 Gone，NULL表示未设置）
	ReplacementModel string      `json:"replacement_model"  orm:"replacement_model"  description:"推荐替代模型名（NULL表示无替代）"`                                              // 推荐替代模型名（NULL表示无替代）
	Capabilities     string      `json:"capabilities"       orm:"capabilities"       description:"模型能力特性（如 vision、function_calling、reasoning 等）"`                   // 模型能力特性（如 vision、function_calling、reasoning 等）
}
