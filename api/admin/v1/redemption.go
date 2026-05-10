package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 兑换码管理 ===

type RedemptionListReq struct {
	g.Meta   `path:"/redemptions" method:"get" mime:"json" tags:"管理后台-兑换码" summary:"兑换码列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
}

type RedemptionItem struct {
	Id           int64       `json:"id"`
	Code         string      `json:"code"`
	Type         string      `json:"type"`
	Value        float64     `json:"value"`
	PlanId       int64       `json:"plan_id"`
	DurationDays int         `json:"duration_days"`
	MaxUses      int         `json:"max_uses"`
	UsedCount    int         `json:"used_count"`
	RedeemedBy   int64       `json:"redeemed_by"`
	RedeemedAt   *gtime.Time `json:"redeemed_at"`
	ExpiresAt    *gtime.Time `json:"expires_at"`
	Status       string      `json:"status"`
	BatchNo      string      `json:"batch_no"`
	CreatedAt    *gtime.Time `json:"created_at"`
	UpdatedAt    *gtime.Time `json:"updated_at"`
}

type RedemptionListRes struct {
	List     []*RedemptionItem `json:"list"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

type RedemptionCreateReq struct {
	g.Meta       `path:"/redemptions" method:"post" mime:"json" tags:"管理后台-兑换码" summary:"批量创建兑换码"`
	Count        int     `json:"count" v:"required|min:1|max:1000"`
	Type         string  `json:"type" v:"required|in:quota,plan,duration"`
	Value        float64 `json:"value"`
	PlanID       int64   `json:"plan_id"`
	DurationDays int     `json:"duration_days"`
}

type RedemptionCreateRes struct {
	Created int `json:"created"`
}

type RedemptionDisableReq struct {
	g.Meta `path:"/redemptions/{id}/disable" method:"put" mime:"json" tags:"管理后台-兑换码" summary:"禁用兑换码"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type RedemptionDisableRes struct{}

type RedemptionUsagesReq struct {
	g.Meta       `path:"/redemptions/usages" method:"get" mime:"json" tags:"管理后台-兑换码" summary:"兑换码使用记录"`
	RedemptionId int64 `json:"redemption_id" in:"query" d:"0"`
	TenantId     int64 `json:"tenant_id" in:"query" d:"0"`
	Page         int   `json:"page" in:"query" d:"1"`
	PageSize     int   `json:"page_size" in:"query" d:"20"`
}

type RedemptionUsageItem struct {
	Id            int64       `json:"id"`
	RedemptionId  int64       `json:"redemption_id"`
	Code          string      `json:"code"`
	TenantId      int64       `json:"tenant_id"`
	TenantName    string      `json:"tenant_name"`
	UserId        int64       `json:"user_id"`
	Username      string      `json:"username"`
	Type          string      `json:"type"`
	Value         float64     `json:"value"`
	TransactionId int64       `json:"transaction_id"`
	CreatedAt     *gtime.Time `json:"created_at"`
}

type RedemptionUsagesRes struct {
	List     []*RedemptionUsageItem `json:"list"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// RedemptionExportReq 导出兑换码列表请求
type RedemptionExportReq struct {
	g.Meta `path:"/redemptions/export" method:"get" mime:"json" tags:"管理后台-兑换码" summary:"导出兑换码列表"`
	Format string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Status string `json:"status" in:"query" dc:"状态筛选"`
}

type RedemptionExportRes struct{}
