// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
)

// TntTenantLevelConfigs is the golang structure for table tnt_tenant_level_configs.
type TntTenantLevelConfigs struct {
	Id                          int64       `json:"id"                            orm:"id"                            description:""`                     //
	Level                       int         `json:"level"                         orm:"level"                         description:"等级号（1, 2, 3...）"`      // 等级号（1, 2, 3...）
	Name                        string      `json:"name"                          orm:"name"                          description:"等级名称"`                 // 等级名称
	CumulativeRechargeThreshold float64     `json:"cumulative_recharge_threshold" orm:"cumulative_recharge_threshold" description:"累计充值阈值（USD），达到此值自动升级"` // 累计充值阈值（USD），达到此值自动升级
	MaxMembers                  int         `json:"max_members"                   orm:"max_members"                   description:"该等级最大成员数"`             // 该等级最大成员数
	MaxConcurrency              int         `json:"max_concurrency"               orm:"max_concurrency"               description:"该等级最大并发数，0=无限"`        // 该等级最大并发数，0=无限
	PriceMultiplier             float64     `json:"price_multiplier"              orm:"price_multiplier"              description:"价格乘数（折扣，如 0.9=九折）"`    // 价格乘数（折扣，如 0.9=九折）
	SortOrder                   int         `json:"sort_order"                    orm:"sort_order"                    description:"排序权重"`                 // 排序权重
	CreatedAt                   *gtime.Time `json:"created_at"                    orm:"created_at"                    description:""`                     //
	UpdatedAt                   *gtime.Time `json:"updated_at"                    orm:"updated_at"                    description:""`                     //
}
