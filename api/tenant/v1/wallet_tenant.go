package v1

import "github.com/gogf/gf/v2/frame/g"

// === 租户钱包 ===

type TenantWalletReq struct {
	g.Meta `path:"/wallet" method:"get" mime:"json" tags:"租户控制台-钱包" summary:"钱包信息"`
}

type TenantWalletRes struct {
	Balance          float64 `json:"balance"`
	FrozenBalance    float64 `json:"frozen_balance"`
	AvailableBalance float64 `json:"available_balance"`
	WarningThreshold float64 `json:"warning_threshold"`
	Currency         string  `json:"currency"`
}

type TenantWalletTransactionsReq struct {
	g.Meta   `path:"/wallet/transactions" method:"get" mime:"json" tags:"租户控制台-钱包" summary:"交易记录"`
	Page     int `json:"page" in:"query" d:"1"`
	PageSize int `json:"page_size" in:"query" d:"20"`
}

type TenantWalletTransactionsRes struct {
	List     []map[string]any `json:"list"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// TenantWalletTransactionsExportReq 导出交易记录请求
type TenantWalletTransactionsExportReq struct {
	g.Meta `path:"/wallet/transactions/export" method:"get" mime:"json" tags:"租户控制台-钱包" summary:"导出交易记录"`
	Format string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
}

type TenantWalletTransactionsExportRes struct{}

// TenantWalletFrozenItemsReq 冻结明细请求
type TenantWalletFrozenItemsReq struct {
	g.Meta `path:"/wallet/frozen-items" method:"get" mime:"json" tags:"租户控制台-钱包" summary:"冻结明细"`
}

// TenantWalletFrozenItemsRes 冻结明细响应
type TenantWalletFrozenItemsRes struct {
	Items []FrozenItemDetail `json:"items"`
}

// FrozenItemDetail 单个冻结项
type FrozenItemDetail struct {
	RequestID string  `json:"request_id"`
	ModelName string  `json:"model_name"`
	Amount    float64 `json:"amount"`
	CreatedAt int64   `json:"created_at"`
	Remaining int64   `json:"remaining"`
}
