package v1

import "github.com/gogf/gf/v2/frame/g"

// TenantLevelBenefitsReq 获取等级权益请求
type TenantLevelBenefitsReq struct {
	g.Meta `path:"/level-benefits" method:"get" mime:"json" tags:"租户控制台-组织" summary:"获取等级权益"`
}

type TenantLevelBenefitsRes struct {
	List               []TenantLevelBenefitItem `json:"list"`
	CurrentLevel       int                      `json:"current_level"`
	CurrentLevelName   string                   `json:"current_level_name"`
	CumulativeRecharge float64                  `json:"cumulative_recharge"`
}

type TenantLevelBenefitItem struct {
	Level                       int     `json:"level"`
	Name                        string  `json:"name"`
	CumulativeRechargeThreshold float64 `json:"cumulative_recharge_threshold"`
	MaxMembers                  int     `json:"max_members"`
	MaxConcurrency              int     `json:"max_concurrency"`
	PriceMultiplier             float64 `json:"price_multiplier"`
}
