// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// OrdPromoCodeUsages is the golang structure of table ord_promo_code_usages for DAO operations like Where/Data.
type OrdPromoCodeUsages struct {
	g.Meta         `orm:"table:ord_promo_code_usages, do:true"`
	Id             any         //
	PromoCodeId    any         //
	TenantId       any         //
	OrderId        any         //
	UserId         any         //
	DiscountAmount any         // 实际折扣金额
	CreatedAt      *gtime.Time //
}
