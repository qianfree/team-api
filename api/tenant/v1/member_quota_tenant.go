package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户成员额度管理 ===

type TenantMemberQuotaReq struct {
	g.Meta `path:"/members/{id}/quota" method:"get" mime:"json" tags:"租户控制台-成员" summary:"获取成员额度"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantMemberQuotaRes struct {
	QuotaType   string  `json:"quota_type"`    // none / total / periodic
	QuotaLimit  float64 `json:"quota_limit"`   // 额度上限(USD)
	QuotaUsed   float64 `json:"quota_used"`    // 已使用额度(USD)
	Period      string  `json:"period"`        // day / week / month
	NextResetAt string  `json:"next_reset_at"` // 下次重置时间（仅 periodic）
}

type TenantMemberQuotaSetReq struct {
	g.Meta     `path:"/members/{id}/quota" method:"put" mime:"json" tags:"租户控制台-成员" summary:"设置成员额度"`
	Id         int64   `json:"id" in:"path" v:"required|min:1"`
	QuotaType  string  `json:"quota_type" v:"required|in:none,total,periodic#请选择额度类型|额度类型无效" dc:"额度类型"`
	QuotaLimit float64 `json:"quota_limit" v:"min:0#额度上限不能为负" dc:"额度上限(USD)"`
	Period     string  `json:"period" v:"in:day,week,month#周期类型无效" dc:"周期类型（periodic 时必填）"`
}

type TenantMemberQuotaSetRes struct{}
