package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 套餐管理 ===

type PlanListReq struct {
	g.Meta   `path:"/plans" method:"get" mime:"json" tags:"管理后台-套餐" summary:"套餐列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
}

type PlanItem struct {
	Id                  int64       `json:"id"`
	Name                string      `json:"name"`
	Identifier          string      `json:"identifier"`
	Description         string      `json:"description"`
	Price               float64     `json:"price"`
	Status              string      `json:"status"`
	CreditAmount        float64     `json:"credit_amount"`
	BonusAmount         float64     `json:"bonus_amount"`
	ValidityDays        int         `json:"validity_days"`
	AllowedModels       []string    `json:"allowed_models"`
	PurchaseLimit       int         `json:"purchase_limit"`
	PurchaseLimitPeriod string      `json:"purchase_limit_period"`
	Stock               int         `json:"stock"`
	TotalPurchased      int         `json:"total_purchased"`
	IsRecommended       bool        `json:"is_recommended"`
	SortOrder           int         `json:"sort_order"`
	CreatedAt           *gtime.Time `json:"created_at"`
	UpdatedAt           *gtime.Time `json:"updated_at"`
}

type PlanListRes struct {
	List     []*PlanItem `json:"list"`
	Total    int         `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

type PlanCreateReq struct {
	g.Meta              `path:"/plans" method:"post" mime:"json" tags:"管理后台-套餐" summary:"创建套餐"`
	Name                string   `json:"name" v:"required#请输入名称"`
	Identifier          string   `json:"identifier" v:"required#请输入标识符"`
	Description         string   `json:"description"`
	Price               float64  `json:"price" v:"min:0.01#价格必须大于0.01"`
	CreditAmount        float64  `json:"credit_amount" v:"min:0.01#额度必须大于0.01"`
	BonusAmount         float64  `json:"bonus_amount"`
	ValidityDays        int      `json:"validity_days" v:"min:1#有效天数至少为1"`
	AllowedModels       []string `json:"allowed_models"`
	PurchaseLimit       int      `json:"purchase_limit"`
	PurchaseLimitPeriod string   `json:"purchase_limit_period" v:"in:lifetime,monthly,yearly#限购周期无效"`
	Stock               int      `json:"stock"`
	IsRecommended       bool     `json:"is_recommended"`
	SortOrder           int      `json:"sort_order"`
}

type PlanCreateRes struct {
	ID int64 `json:"id"`
}

type PlanDetailReq struct {
	g.Meta `path:"/plans/{id}" method:"get" mime:"json" tags:"管理后台-套餐" summary:"套餐详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type PlanDetailRes struct {
	Data *PlanItem `json:"data"`
}

type PlanUpdateReq struct {
	g.Meta `path:"/plans/{id}" method:"put" mime:"json" tags:"管理后台-套餐" summary:"更新套餐"`
	Id     int64                  `json:"id" in:"path" v:"required|min:1"`
	Update map[string]interface{} `json:"update" v:"required"`
}

type PlanUpdateRes struct{}

type PlanArchiveReq struct {
	g.Meta `path:"/plans/{id}" method:"delete" mime:"json" tags:"管理后台-套餐" summary:"下架套餐"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type PlanArchiveRes struct{}

type PlanToggleRecommendReq struct {
	g.Meta `path:"/plans/{id}/toggle-recommend" method:"put" mime:"json" tags:"管理后台-套餐" summary:"切换推荐"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type PlanToggleRecommendRes struct{}

// PlanExportReq 导出套餐列表请求
type PlanExportReq struct {
	g.Meta `path:"/plans/export" method:"get" mime:"json" tags:"管理后台-套餐" summary:"导出套餐列表"`
	Format string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Status string `json:"status" in:"query" dc:"状态筛选"`
}

type PlanExportRes struct{}
