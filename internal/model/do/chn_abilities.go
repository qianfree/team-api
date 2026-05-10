// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// ChnAbilities is the golang structure of table chn_abilities for DAO operations like Where/Data.
type ChnAbilities struct {
	g.Meta        `orm:"table:chn_abilities, do:true"`
	Id            any         // 主键ID
	ChannelId     any         // 关联渠道ID
	ModelName     any         // 平台标准模型名（用户请求使用的模型名）
	UpstreamModel any         // 上游实际模型名（与平台标准名不同时需要映射，如平台名 gpt-4 → 上游名 gpt-4-0314）
	Enabled       any         // 是否启用该模型能力
	CreatedAt     *gtime.Time // 创建时间
	UpdatedAt     *gtime.Time // 更新时间
}
