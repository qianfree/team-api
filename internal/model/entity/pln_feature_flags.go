// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PlnFeatureFlags is the golang structure for table pln_feature_flags.
type PlnFeatureFlags struct {
	Id             int64       `json:"id"              orm:"id"              description:"主键ID"`                                  // 主键ID
	FeatureKey     string      `json:"feature_key"     orm:"feature_key"     description:"功能标识（如 api_docs, export_csv）"`          // 功能标识（如 api_docs, export_csv）
	Description    string      `json:"description"     orm:"description"     description:"功能描述"`                                  // 功能描述
	DefaultEnabled bool        `json:"default_enabled" orm:"default_enabled" description:"默认是否启用"`                                // 默认是否启用
	Enabled        bool        `json:"enabled"         orm:"enabled"         description:"当前是否启用（计算后的最终值）"`                       // 当前是否启用（计算后的最终值）
	Source         string      `json:"source"          orm:"source"          description:"来源：plan（套餐）/ tenant（租户覆盖）/ manual（手动）"` // 来源：plan（套餐）/ tenant（租户覆盖）/ manual（手动）
	SourceId       int64       `json:"source_id"       orm:"source_id"       description:"来源ID（plan_id 或 tenant_id）"`             // 来源ID（plan_id 或 tenant_id）
	TenantId       int64       `json:"tenant_id"       orm:"tenant_id"       description:"关联租户ID（租户级覆盖时使用）"`                      // 关联租户ID（租户级覆盖时使用）
	PlanId         int64       `json:"plan_id"         orm:"plan_id"         description:"关联套餐ID（套餐级配置时使用）"`                      // 关联套餐ID（套餐级配置时使用）
	CreatedAt      *gtime.Time `json:"created_at"      orm:"created_at"      description:"创建时间"`                                  // 创建时间
	UpdatedAt      *gtime.Time `json:"updated_at"      orm:"updated_at"      description:"更新时间"`                                  // 更新时间
}
