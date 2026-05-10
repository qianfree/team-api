package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 租户兑换码 / 优惠码 ===

type TenantRedeemCodeReq struct {
	g.Meta `path:"/redemptions/redeem" method:"post" mime:"json" tags:"租户控制台-优惠" summary:"兑换码"`
	Code   string `json:"code" v:"required"`
}

type TenantRedeemCodeRes struct {
	Data map[string]any `json:"data"`
}

type TenantValidatePromoCodeReq struct {
	g.Meta `path:"/promo-codes/validate" method:"post" mime:"json" tags:"租户控制台-优惠" summary:"验证优惠码"`
	Code   string  `json:"code" v:"required"`
	Amount float64 `json:"amount"`
}

type TenantValidatePromoCodeRes struct {
	Data map[string]any `json:"data"`
}

type TenantRedemptionUsagesReq struct {
	g.Meta   `path:"/redemptions/usages" method:"get" mime:"json" tags:"租户控制台-优惠" summary:"我的兑换历史"`
	Page     int `json:"page" in:"query" d:"1"`
	PageSize int `json:"page_size" in:"query" d:"20"`
}

type TenantRedemptionUsageItem struct {
	Id            int64       `json:"id"`
	RedemptionId  int64       `json:"redemption_id"`
	Code          string      `json:"code"`
	Type          string      `json:"type"`
	Value         float64     `json:"value"`
	TransactionId int64       `json:"transaction_id"`
	CreatedAt     *gtime.Time `json:"created_at"`
}

type TenantRedemptionUsagesRes struct {
	List     []*TenantRedemptionUsageItem `json:"list"`
	Total    int                          `json:"total"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}
