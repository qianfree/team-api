// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

// OrdPromoCodeUsages is the golang structure for table ord_promo_code_usages.
type OrdPromoCodeUsages struct {
	Id             int64           `json:"id"              orm:"id"              description:""`       //
	PromoCodeId    int64           `json:"promo_code_id"   orm:"promo_code_id"   description:""`       //
	TenantId       int64           `json:"tenant_id"       orm:"tenant_id"       description:""`       //
	OrderId        int64           `json:"order_id"        orm:"order_id"        description:""`       //
	UserId         int64           `json:"user_id"         orm:"user_id"         description:""`       //
	DiscountAmount decimal.Decimal `json:"discount_amount" orm:"discount_amount" description:"实际折扣金额"` // 实际折扣金额
	CreatedAt      *gtime.Time     `json:"created_at"      orm:"created_at"      description:""`       //
}
