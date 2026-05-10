// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnChannelAffinities is the golang structure for table chn_channel_affinities.
type ChnChannelAffinities struct {
	Id        int64       `json:"id"         orm:"id"         description:"主键ID"`               // 主键ID
	TenantId  int64       `json:"tenant_id"  orm:"tenant_id"  description:"租户ID"`               // 租户ID
	UserId    int64       `json:"user_id"    orm:"user_id"    description:"用户ID"`               // 用户ID
	ModelName string      `json:"model_name" orm:"model_name" description:"模型名"`                // 模型名
	ChannelId int64       `json:"channel_id" orm:"channel_id" description:"绑定的渠道ID"`            // 绑定的渠道ID
	HitCount  int         `json:"hit_count"  orm:"hit_count"  description:"命中次数（同一渠道连续成功次数）"`   // 命中次数（同一渠道连续成功次数）
	ExpiresAt *gtime.Time `json:"expires_at" orm:"expires_at" description:"过期时间（默认 1800 秒后过期）"` // 过期时间（默认 1800 秒后过期）
	CreatedAt *gtime.Time `json:"created_at" orm:"created_at" description:"创建时间"`               // 创建时间
	UpdatedAt *gtime.Time `json:"updated_at" orm:"updated_at" description:"更新时间"`               // 更新时间
}
