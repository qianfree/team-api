package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 优惠码管理 ===

type PromoCodeListReq struct {
	g.Meta   `path:"/promo-codes" method:"get" mime:"json" tags:"管理后台-优惠码" summary:"优惠码列表"`
	Page     int `json:"page" in:"query" d:"1"`
	PageSize int `json:"page_size" in:"query" d:"20"`
}

type PromoCodeItem struct {
	Id            int64       `json:"id"`
	Code          string      `json:"code"`
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	DiscountValue float64     `json:"discount_value"`
	MinAmount     float64     `json:"min_amount"`
	MaxDiscount   float64     `json:"max_discount"`
	TotalCount    int         `json:"total_count"`
	UsedCount     int         `json:"used_count"`
	PerUserLimit  int         `json:"per_user_limit"`
	ValidFrom     *gtime.Time `json:"valid_from"`
	ValidTo       *gtime.Time `json:"valid_to"`
	PlanIds       []int64     `json:"plan_ids"`
	Status        string      `json:"status"`
	CreatedAt     *gtime.Time `json:"created_at"`
	UpdatedAt     *gtime.Time `json:"updated_at"`
}

type PromoCodeListRes struct {
	List     []*PromoCodeItem `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

type PromoCodeCreateReq struct {
	g.Meta        `path:"/promo-codes" method:"post" mime:"json" tags:"管理后台-优惠码" summary:"创建优惠码"`
	Name          string      `json:"name" v:"required#名称不能为空" dc:"名称"`
	Code          string      `json:"code" dc:"优惠码文本（留空自动生成）"`
	Type          string      `json:"type" v:"required|in:percentage,fixed#类型不能为空|类型仅支持 percentage/fixed" dc:"类型：percentage（折扣百分比）/ fixed（立减固定金额）"`
	DiscountValue float64     `json:"discount_value" v:"required|min:0#折扣值不能为空|折扣值不能为负" dc:"折扣值（百分比 0-100，立减为金额）"`
	MinAmount     float64     `json:"min_amount" d:"0" v:"min:0#最低金额不能为负" dc:"最低订单金额"`
	MaxDiscount   float64     `json:"max_discount" d:"0" v:"min:0#最大折扣不能为负" dc:"最大折扣金额（0=不限）"`
	TotalCount    int64       `json:"total_count" d:"0" v:"min:0#总次数不能为负" dc:"总使用次数（0=不限）"`
	PerUserLimit  int64       `json:"per_user_limit" d:"1" v:"min:0#每人限用不能为负" dc:"每人限用次数"`
	Status        string      `json:"status" d:"active" v:"in:active,disabled#状态仅支持 active/disabled" dc:"状态"`
	ValidFrom     *gtime.Time `json:"valid_from" dc:"有效期开始"`
	ValidTo       *gtime.Time `json:"valid_to" dc:"有效期结束"`
	PlanIds       []int64     `json:"plan_ids" dc:"适用套餐ID数组（空=全部）"`
}

type PromoCodeCreateRes struct {
	ID int64 `json:"id"`
}

type PromoCodeUpdateReq struct {
	g.Meta        `path:"/promo-codes/{id}" method:"put" mime:"json" tags:"管理后台-优惠码" summary:"更新优惠码"`
	Id            int64       `json:"id" in:"path" v:"required|min:1"`
	Name          *string     `json:"name" dc:"名称（留空不更新）"`
	Type          *string     `json:"type" v:"in:percentage,fixed#类型仅支持 percentage/fixed" dc:"类型（留空不更新）"`
	DiscountValue *float64    `json:"discount_value" v:"min:0#折扣值不能为负" dc:"折扣值（留空不更新）"`
	MinAmount     *float64    `json:"min_amount" v:"min:0#最低金额不能为负" dc:"最低订单金额（留空不更新）"`
	MaxDiscount   *float64    `json:"max_discount" v:"min:0#最大折扣不能为负" dc:"最大折扣金额（留空不更新）"`
	TotalCount    *int64      `json:"total_count" v:"min:0#总次数不能为负" dc:"总使用次数（留空不更新）"`
	PerUserLimit  *int64      `json:"per_user_limit" v:"min:0#每人限用不能为负" dc:"每人限用次数（留空不更新）"`
	Status        *string     `json:"status" v:"in:active,disabled#状态仅支持 active/disabled" dc:"状态（留空不更新）"`
	ValidFrom     *gtime.Time `json:"valid_from" dc:"有效期开始（留空不更新）"`
	ValidTo       *gtime.Time `json:"valid_to" dc:"有效期结束（留空不更新）"`
	PlanIds       []int64     `json:"plan_ids" dc:"适用套餐ID数组"`
}

type PromoCodeUpdateRes struct{}

type PromoCodeUsagesReq struct {
	g.Meta   `path:"/promo-codes/{id}/usages" method:"get" mime:"json" tags:"管理后台-优惠码" summary:"优惠码使用记录"`
	Id       int64 `json:"id" in:"path" v:"required|min:1"`
	Page     int   `json:"page" in:"query" d:"1"`
	PageSize int   `json:"page_size" in:"query" d:"20"`
}

type PromoCodeUsageItem struct {
	Id             int64       `json:"id"`
	PromoCodeId    int64       `json:"promo_code_id"`
	TenantId       int64       `json:"tenant_id"`
	OrderId        int64       `json:"order_id"`
	UserId         int64       `json:"user_id"`
	DiscountAmount float64     `json:"discount_amount"`
	CreatedAt      *gtime.Time `json:"created_at"`
}

type PromoCodeUsagesRes struct {
	List     []*PromoCodeUsageItem `json:"list"`
	Total    int                   `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

// PromoCodeExportReq 导出优惠码列表请求
type PromoCodeExportReq struct {
	g.Meta `path:"/promo-codes/export" method:"get" mime:"json" tags:"管理后台-优惠码" summary:"导出优惠码列表"`
	Format string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
}

type PromoCodeExportRes struct{}
