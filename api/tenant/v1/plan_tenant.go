package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 租户套餐 ===

type TenantPlanItem struct {
	Id            int64   `json:"id"`
	Name          string  `json:"name"`
	Identifier    string  `json:"identifier"`
	Description   string  `json:"description"`
	Price         float64 `json:"price"`
	CreditAmount  float64 `json:"credit_amount"`
	BonusAmount   float64 `json:"bonus_amount"`
	ValidityDays  int     `json:"validity_days"`
	IsRecommended bool    `json:"is_recommended"`
	SortOrder     int     `json:"sort_order"`
}

type TenantPlanListReq struct {
	g.Meta `path:"/plans" method:"get" mime:"json" tags:"租户控制台-套餐" summary:"可购买套餐列表"`
}

type TenantPlanListRes struct {
	List []*TenantPlanItem `json:"list"`
}

// TenantPlanMineReq 我的套餐
type TenantPlanMineReq struct {
	g.Meta `path:"/plans/mine" method:"get" mime:"json" tags:"租户控制台-套餐" summary:"我的套餐"`
}

type TenantPlanMineItem struct {
	Id               int64       `json:"id"`
	PlanId           int64       `json:"plan_id"`
	PlanName         string      `json:"plan_name"`
	Status           string      `json:"status"`
	TotalCredits     float64     `json:"total_credits"`
	RemainingCredits float64     `json:"remaining_credits"`
	StartAt          *gtime.Time `json:"start_at"`
	EndAt            *gtime.Time `json:"end_at"`
	ActivatedAt      *gtime.Time `json:"activated_at"`
	ExpiresAt        *gtime.Time `json:"expires_at"`
	CreatedAt        *gtime.Time `json:"created_at"`
}

type TenantPlanMineRes struct {
	List           []*TenantPlanMineItem `json:"list"`
	TotalRemaining float64               `json:"total_remaining"`
	ActiveCount    int                   `json:"active_count"`
}

// TenantPlanOrderCreateReq 购买套餐创建订单
type TenantPlanOrderCreateReq struct {
	g.Meta         `path:"/plans/orders" method:"post" mime:"json" tags:"租户控制台-套餐" summary:"购买套餐"`
	PlanId         int64  `json:"plan_id" v:"required|min:1#请选择套餐"`
	PaymentChannel string `json:"payment_channel" v:"required#请选择支付渠道"`
	PaymentMethod  string `json:"payment_method" v:"required#请选择支付方式"`
}

type TenantPlanOrderCreateRes struct {
	OrderId    int64  `json:"order_id"`
	OrderNo    string `json:"order_no"`
	PaymentUrl string `json:"payment_url"`
}
