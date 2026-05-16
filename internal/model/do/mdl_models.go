// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// MdlModels is the golang structure of table mdl_models for DAO operations like Where/Data.
type MdlModels struct {
	g.Meta           `orm:"table:mdl_models, do:true"`
	Id               any         // 主键ID
	ModelId          any         // 模型唯一标识（如 gpt-4o、claude-3-5-sonnet）
	ModelName        any         // 模型显示名称（如 GPT-4o、Claude 3.5 Sonnet）
	Category         any         // 模型分类：chat（对话）/ embedding（嵌入）/ image（图像）/ audio（音频）/ rerank（重排序）
	Status           any         // 状态：active（可用）/ deprecated（已废弃）/ offline（已下线）
	MaxContextTokens any         // 最大上下文 token 数
	MaxOutputTokens  any         // 最大输出 token 数
	Description      any         // 模型描述
	Tags             []string    // 标签（如 reasoning、vision、function_calling）
	CreatedAt        *gtime.Time // 创建时间
	UpdatedAt        *gtime.Time // 更新时间
	DeprecatedAt     *gtime.Time // 标记弃用的时间（NULL表示未弃用）
	SunsetDate       *gtime.Time // 计划下线日期（到达后返回410 Gone，NULL表示未设置）
	ReplacementModel any         // 推荐替代模型名（NULL表示无替代）
	Capabilities     any         // 模型能力特性（如 vision、function_calling、reasoning 等）
}
