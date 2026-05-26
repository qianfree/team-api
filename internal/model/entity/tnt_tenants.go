// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TntTenants is the golang structure for table tnt_tenants.
type TntTenants struct {
	Id                  int64       `json:"id"                    orm:"id"                    description:"主键ID"`                                                                                                      // 主键ID
	Name                string      `json:"name"                  orm:"name"                  description:"租户显示名称（如公司名）"`                                                                                              // 租户显示名称（如公司名）
	Code                string      `json:"code"                  orm:"code"                  description:"租户代码（唯一标识，用于 RAM 账号格式 username@tenant_code）"`                                                               // 租户代码（唯一标识，用于 RAM 账号格式 username@tenant_code）
	LogoUrl             string      `json:"logo_url"              orm:"logo_url"              description:"租户 Logo URL"`                                                                                               // 租户 Logo URL
	OwnerUserId         int64       `json:"owner_user_id"         orm:"owner_user_id"         description:"所有者用户ID（关联 tnt_users.id）"`                                                                                  // 所有者用户ID（关联 tnt_users.id）
	Status              string      `json:"status"                orm:"status"                description:"状态：trial（试用）/ active（活跃）/ past_due（逾期）/ frozen（冻结）/ terminated（已终止）/ free（免费版）/ suspended（暂停）/ closed（关闭）"` // 状态：trial（试用）/ active（活跃）/ past_due（逾期）/ frozen（冻结）/ terminated（已终止）/ free（免费版）/ suspended（暂停）/ closed（关闭）
	MaxMembers          int         `json:"max_members"           orm:"max_members"           description:"最大成员数上限"`                                                                                                   // 最大成员数上限
	Settings            string      `json:"settings"              orm:"settings"              description:"租户配置（JSONB）：通知偏好、安全策略、IP 白名单等"`                                                                             // 租户配置（JSONB）：通知偏好、安全策略、IP 白名单等
	CreatedAt           *gtime.Time `json:"created_at"            orm:"created_at"            description:"创建时间"`                                                                                                      // 创建时间
	UpdatedAt           *gtime.Time `json:"updated_at"            orm:"updated_at"            description:"更新时间"`                                                                                                      // 更新时间
	TrialEndsAt         *gtime.Time `json:"trial_ends_at"         orm:"trial_ends_at"         description:"试用期结束时间"`                                                                                                   // 试用期结束时间
	GracePeriodEndsAt   *gtime.Time `json:"grace_period_ends_at"  orm:"grace_period_ends_at"  description:"宽限期结束时间（套餐到期后 7 天）"`                                                                                        // 宽限期结束时间（套餐到期后 7 天）
	FrozenAt            *gtime.Time `json:"frozen_at"             orm:"frozen_at"             description:"冻结时间"`                                                                                                      // 冻结时间
	ClosingRequestedAt  *gtime.Time `json:"closing_requested_at"  orm:"closing_requested_at"  description:"主动申请注销时间（7 天冷静期）"`                                                                                          // 主动申请注销时间（7 天冷静期）
	DataRemovalAt       *gtime.Time `json:"data_removal_at"       orm:"data_removal_at"       description:"数据清除时间（冻结 30 天后）"`                                                                                          // 数据清除时间（冻结 30 天后）
	MaxConcurrency      int         `json:"max_concurrency"       orm:"max_concurrency"       description:"租户总并发上限（0表示不限制）"`                                                                                           // 租户总并发上限（0表示不限制）
	DefaultChannelScope string      `json:"default_channel_scope" orm:"default_channel_scope" description:"默认渠道范围（NULL或[]表示全部可用，否则为channel_id数组）"`                                                                     // 默认渠道范围（NULL或[]表示全部可用，否则为channel_id数组）
	Level               int         `json:"level"                 orm:"level"                 description:"当前等级（对应 tnt_tenant_level_configs.level）"`                                                                   // 当前等级（对应 tnt_tenant_level_configs.level）
}
