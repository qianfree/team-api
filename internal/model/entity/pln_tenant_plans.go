// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// PlnTenantPlans is the golang structure for table pln_tenant_plans.
type PlnTenantPlans struct {
	Id                 int64       `json:"id"                   orm:"id"                   description:"主键ID"`                                                       // 主键ID
	TenantId           int64       `json:"tenant_id"            orm:"tenant_id"            description:"租户ID"`                                                       // 租户ID
	PlanId             int64       `json:"plan_id"              orm:"plan_id"              description:"套餐ID"`                                                       // 套餐ID
	Status             string      `json:"status"               orm:"status"               description:"状态：pending（待生效）/ active（生效中）/ expired（已过期）/ cancelled（已取消）"` // 状态：pending（待生效）/ active（生效中）/ expired（已过期）/ cancelled（已取消）
	StartAt            *gtime.Time `json:"start_at"             orm:"start_at"             description:"生效起始时间"`                                                     // 生效起始时间
	EndAt              *gtime.Time `json:"end_at"               orm:"end_at"               description:"到期时间"`                                                       // 到期时间
	AutoRenew          bool        `json:"auto_renew"           orm:"auto_renew"           description:"是否自动续费"`                                                     // 是否自动续费
	MonthlyQuotaTokens int64       `json:"monthly_quota_tokens" orm:"monthly_quota_tokens" description:"月度 Token 配额快照"`                                              // 月度 Token 配额快照
	UsedTokens         int64       `json:"used_tokens"          orm:"used_tokens"          description:"本月已使用 Token"`                                                // 本月已使用 Token
	LastResetAt        *gtime.Time `json:"last_reset_at"        orm:"last_reset_at"        description:"上次配额重置时间"`                                                   // 上次配额重置时间
	CancelledAt        *gtime.Time `json:"cancelled_at"         orm:"cancelled_at"         description:"取消时间"`                                                       // 取消时间
	CreatedAt          *gtime.Time `json:"created_at"           orm:"created_at"           description:"创建时间"`                                                       // 创建时间
	UpdatedAt          *gtime.Time `json:"updated_at"           orm:"updated_at"           description:"更新时间"`                                                       // 更新时间
}
