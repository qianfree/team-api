// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TntTenants is the golang structure of table tnt_tenants for DAO operations like Where/Data.
type TntTenants struct {
	g.Meta              `orm:"table:tnt_tenants, do:true"`
	Id                  any         // 主键ID
	Name                any         // 租户显示名称（如公司名）
	Code                any         // 租户代码（唯一标识，用于 RAM 账号格式 username@tenant_code）
	LogoUrl             any         // 租户 Logo URL
	OwnerUserId         any         // 所有者用户ID（关联 tnt_users.id）
	Status              any         // 状态：trial（试用）/ active（活跃）/ past_due（逾期）/ frozen（冻结）/ terminated（已终止）/ free（免费版）/ suspended（暂停）/ closed（关闭）
	MaxMembers          any         // 最大成员数上限（NULL表示跟随等级配置）
	Settings            any         // 租户配置（JSONB）：通知偏好、安全策略、IP 白名单等
	CreatedAt           *gtime.Time // 创建时间
	UpdatedAt           *gtime.Time // 更新时间
	TrialEndsAt         *gtime.Time // 试用期结束时间
	GracePeriodEndsAt   *gtime.Time // 宽限期结束时间（套餐到期后 7 天）
	FrozenAt            *gtime.Time // 冻结时间
	ClosingRequestedAt  *gtime.Time // 主动申请注销时间（7 天冷静期）
	DataRemovalAt       *gtime.Time // 数据清除时间（冻结 30 天后）
	MaxConcurrency      any         // 租户总并发上限（NULL表示跟随等级配置，0表示不限制）
	DefaultChannelScope any         // 默认渠道范围（NULL或[]表示全部可用，否则为channel_id数组）
	Level               any         // 当前等级（对应 tnt_tenant_level_configs.level）
}
