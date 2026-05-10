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
	g.Meta `path:"/promo-codes" method:"post" mime:"json" tags:"管理后台-优惠码" summary:"创建优惠码"`
	Data   map[string]interface{} `json:"data" v:"required"`
}

type PromoCodeCreateRes struct {
	ID int64 `json:"id"`
}

type PromoCodeUpdateReq struct {
	g.Meta `path:"/promo-codes/{id}" method:"put" mime:"json" tags:"管理后台-优惠码" summary:"更新优惠码"`
	Id     int64                  `json:"id" in:"path" v:"required|min:1"`
	Update map[string]interface{} `json:"update" v:"required"`
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
