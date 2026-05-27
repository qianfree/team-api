package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 租户等级配置管理 ===

type TenantLevelConfigItem struct {
	Id                          int64       `json:"id"`
	Level                       int         `json:"level"`
	Name                        string      `json:"name"`
	CumulativeRechargeThreshold float64     `json:"cumulative_recharge_threshold"`
	MaxMembers                  int         `json:"max_members"`
	MaxConcurrency              int         `json:"max_concurrency"`
	PriceMultiplier             float64     `json:"price_multiplier"`
	SortOrder                   int         `json:"sort_order"`
	CreatedAt                   *gtime.Time `json:"created_at"`
	UpdatedAt                   *gtime.Time `json:"updated_at"`
}

type TenantLevelConfigListReq struct {
	g.Meta `path:"/tenant-level-configs" method:"get" mime:"json" tags:"管理后台-租户等级" summary:"等级配置列表"`
}

type TenantLevelConfigListRes struct {
	List []*TenantLevelConfigItem `json:"list"`
}

type TenantLevelConfigCreateReq struct {
	g.Meta                      `path:"/tenant-level-configs" method:"post" mime:"json" tags:"管理后台-租户等级" summary:"创建等级配置"`
	Level                       int     `json:"level" v:"required|min:1#请输入等级号|等级号最小为1"`
	Name                        string  `json:"name" v:"required#请输入等级名称"`
	CumulativeRechargeThreshold float64 `json:"cumulative_recharge_threshold" v:"required|min:0#请输入累计充值阈值|阈值不能为负数"`
	MaxMembers                  int     `json:"max_members" v:"min:0#最大成员数不能为负数"`
	MaxConcurrency              int     `json:"max_concurrency" v:"min:0#最大并发数不能为负数"`
	PriceMultiplier             float64 `json:"price_multiplier" d:"1.0000"`
	SortOrder                   int     `json:"sort_order" d:"0"`
}

type TenantLevelConfigCreateRes struct {
	ID int64 `json:"id"`
}

type TenantLevelConfigUpdateReq struct {
	g.Meta                      `path:"/tenant-level-configs/{id}" method:"put" mime:"json" tags:"管理后台-租户等级" summary:"更新等级配置"`
	Id                          int64    `json:"id" in:"path" v:"required|min:1"`
	Name                        *string  `json:"name" dc:"等级名称"`
	CumulativeRechargeThreshold *float64 `json:"cumulative_recharge_threshold" dc:"累计充值阈值"`
	MaxMembers                  *int     `json:"max_members" dc:"最大成员数"`
	MaxConcurrency              *int     `json:"max_concurrency" dc:"最大并发数"`
	PriceMultiplier             *float64 `json:"price_multiplier" dc:"价格乘数"`
	SortOrder                   *int     `json:"sort_order" dc:"排序权重"`
}

type TenantLevelConfigUpdateRes struct{}

type TenantLevelConfigDeleteReq struct {
	g.Meta `path:"/tenant-level-configs/{id}" method:"delete" mime:"json" tags:"管理后台-租户等级" summary:"删除等级配置"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantLevelConfigDeleteRes struct{}
