// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// PlnFeatureFlags is the golang structure of table pln_feature_flags for DAO operations like Where/Data.
type PlnFeatureFlags struct {
	g.Meta         `orm:"table:pln_feature_flags, do:true"`
	Id             any         // 主键ID
	FeatureKey     any         // 功能标识（如 api_docs, export_csv）
	Description    any         // 功能描述
	DefaultEnabled any         // 默认是否启用
	Enabled        any         // 当前是否启用（计算后的最终值）
	Source         any         // 来源：plan（套餐）/ tenant（租户覆盖）/ manual（手动）
	SourceId       any         // 来源ID（plan_id 或 tenant_id）
	TenantId       any         // 关联租户ID（租户级覆盖时使用）
	PlanId         any         // 关联套餐ID（套餐级配置时使用）
	CreatedAt      *gtime.Time // 创建时间
	UpdatedAt      *gtime.Time // 更新时间
}
