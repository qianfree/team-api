// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnChannelAffinities is the golang structure of table chn_channel_affinities for DAO operations like Where/Data.
type ChnChannelAffinities struct {
	g.Meta    `orm:"table:chn_channel_affinities, do:true"`
	Id        any         // 主键ID
	TenantId  any         // 租户ID
	UserId    any         // 用户ID
	ModelName any         // 模型名
	ChannelId any         // 绑定的渠道ID
	HitCount  any         // 命中次数（同一渠道连续成功次数）
	ExpiresAt *gtime.Time // 过期时间（默认 1800 秒后过期）
	CreatedAt *gtime.Time // 创建时间
	UpdatedAt *gtime.Time // 更新时间
}
