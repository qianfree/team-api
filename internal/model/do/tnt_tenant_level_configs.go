// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// TntTenantLevelConfigs is the golang structure of table tnt_tenant_level_configs for DAO operations like Where/Data.
type TntTenantLevelConfigs struct {
	g.Meta                      `orm:"table:tnt_tenant_level_configs, do:true"`
	Id                          any         //
	Level                       any         // 等级号（1, 2, 3...）
	Name                        any         // 等级名称
	CumulativeRechargeThreshold any         // 累计充值阈值（USD），达到此值自动升级
	MaxMembers                  any         // 该等级最大成员数
	MaxConcurrency              any         // 该等级最大并发数，0=无限
	PriceMultiplier             any         // 价格乘数（折扣，如 0.9=九折）
	SortOrder                   any         // 排序权重
	CreatedAt                   *gtime.Time //
	UpdatedAt                   *gtime.Time //
}
