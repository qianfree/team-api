package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 租户套餐 ===

type TenantPlanItem struct {
	Id                 int64    `json:"id"`
	Name               string   `json:"name"`
	Identifier         string   `json:"identifier"`
	Description        string   `json:"description"`
	MonthlyPrice       float64  `json:"monthly_price"`
	YearlyPrice        float64  `json:"yearly_price"`
	MonthlyQuotaTokens int64    `json:"monthly_quota_tokens"`
	AllowedModels      []string `json:"allowed_models"`
	IsRecommended      bool     `json:"is_recommended"`
	SortOrder          int      `json:"sort_order"`
}

type TenantPlanListReq struct {
	g.Meta `path:"/plans" method:"get" mime:"json" tags:"租户控制台-套餐" summary:"套餐列表"`
}

type TenantPlanListRes struct {
	List []*TenantPlanItem `json:"list"`
}

type TenantPlanCurrentReq struct {
	g.Meta `path:"/plan/current" method:"get" mime:"json" tags:"租户控制台-套餐" summary:"当前套餐"`
}

type TenantPlanCurrentRes struct {
	Id                 int64       `json:"id"`
	TenantId           int64       `json:"tenant_id"`
	PlanId             int64       `json:"plan_id"`
	Status             string      `json:"status"`
	StartAt            *gtime.Time `json:"start_at"`
	EndAt              *gtime.Time `json:"end_at"`
	AutoRenew          bool        `json:"auto_renew"`
	MonthlyQuotaTokens int64       `json:"monthly_quota_tokens"`
	UsedTokens         int64       `json:"used_tokens"`
	LastResetAt        *gtime.Time `json:"last_reset_at"`
	Name               string      `json:"name"`
	Identifier         string      `json:"identifier"`
	Description        string      `json:"description"`
	MonthlyPrice       float64     `json:"monthly_price"`
	YearlyPrice        float64     `json:"yearly_price"`
}

type TenantPlanCancelAutoRenewReq struct {
	g.Meta `path:"/plan/cancel-auto-renew" method:"put" mime:"json" tags:"租户控制台-套餐" summary:"取消自动续费"`
}

type TenantPlanCancelAutoRenewRes struct{}
