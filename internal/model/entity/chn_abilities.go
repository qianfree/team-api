// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnAbilities is the golang structure for table chn_abilities.
type ChnAbilities struct {
	Id            int64       `json:"id"             orm:"id"             description:"主键ID"`                                               // 主键ID
	ChannelId     int64       `json:"channel_id"     orm:"channel_id"     description:"关联渠道ID"`                                             // 关联渠道ID
	ModelName     string      `json:"model_name"     orm:"model_name"     description:"平台标准模型名（用户请求使用的模型名）"`                                // 平台标准模型名（用户请求使用的模型名）
	UpstreamModel string      `json:"upstream_model" orm:"upstream_model" description:"上游实际模型名（与平台标准名不同时需要映射，如平台名 gpt-4 → 上游名 gpt-4-0314）"` // 上游实际模型名（与平台标准名不同时需要映射，如平台名 gpt-4 → 上游名 gpt-4-0314）
	Enabled       bool        `json:"enabled"        orm:"enabled"        description:"是否启用该模型能力"`                                          // 是否启用该模型能力
	CreatedAt     *gtime.Time `json:"created_at"     orm:"created_at"     description:"创建时间"`                                               // 创建时间
	UpdatedAt     *gtime.Time `json:"updated_at"     orm:"updated_at"     description:"更新时间"`                                               // 更新时间
}
