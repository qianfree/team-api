package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 订单管理（管理后台） ===

type OrderListReq struct {
	g.Meta   `path:"/orders" method:"get" mime:"json" tags:"管理后台-订单" summary:"订单列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
	TenantID string `json:"tenant_id" in:"query"`
}

type OrderItem struct {
	Id             int64       `json:"id"`
	OrderNo        string      `json:"order_no"`
	TenantId       int64       `json:"tenant_id"`
	UserId         int64       `json:"user_id"`
	OrderType      string      `json:"order_type"`
	PlanId         int64       `json:"plan_id"`
	Amount         float64     `json:"amount"`
	DiscountAmount float64     `json:"discount_amount"`
	FinalAmount    float64     `json:"final_amount"`
	Currency       string      `json:"currency"`
	PaymentChannel string      `json:"payment_channel"`
	PaymentMethod  string      `json:"payment_method"`
	PaymentNo      string      `json:"payment_no"`
	Status         string      `json:"status"`
	PaidAt         *gtime.Time `json:"paid_at"`
	FulfilledAt    *gtime.Time `json:"fulfilled_at"`
	ExpiredAt      *gtime.Time `json:"expired_at"`
	CancelledAt    *gtime.Time `json:"cancelled_at"`
	RelatedOrderId int64       `json:"related_order_id"`
	Description    string      `json:"description"`
	CreatedAt      *gtime.Time `json:"created_at"`
	UpdatedAt      *gtime.Time `json:"updated_at"`
}

type OrderListRes struct {
	List     []*OrderItem `json:"list"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

type OrderDetailReq struct {
	g.Meta `path:"/orders/{id}" method:"get" mime:"json" tags:"管理后台-订单" summary:"订单详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type OrderDetailRes struct {
	Data map[string]any `json:"data"`
}

type OrderRefundReq struct {
	g.Meta `path:"/orders/{id}/refund" method:"post" mime:"json" tags:"管理后台-订单" summary:"发起退款"`
	Id     int64  `json:"id" in:"path" v:"required|min:1"`
	Reason string `json:"reason"`
}

type OrderRefundRes struct {
	RefundAmount float64 `json:"refund_amount"`
}

// OrderRefundPreviewReq 退款预览（计算可退金额）
type OrderRefundPreviewReq struct {
	g.Meta `path:"/orders/{id}/refund-preview" method:"get" mime:"json" tags:"管理后台-订单" summary:"退款预览"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type OrderRefundPreviewRes struct {
	CanRefund    bool    `json:"can_refund"`
	RefundAmount float64 `json:"refund_amount"`
	OrderType    string  `json:"order_type"`
	OrderStatus  string  `json:"order_status"`
	Message      string  `json:"message,omitempty"`
}

type OrderCompleteReq struct {
	g.Meta `path:"/orders/{id}/complete" method:"post" mime:"json" tags:"管理后台-订单" summary:"手动完成订单"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type OrderCompleteRes struct{}

// OrderExportReq 导出订单列表请求
type OrderExportReq struct {
	g.Meta   `path:"/orders/export" method:"get" mime:"json" tags:"管理后台-订单" summary:"导出订单列表"`
	Format   string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Status   string `json:"status" in:"query" dc:"状态筛选"`
	TenantID string `json:"tenant_id" in:"query" dc:"租户ID筛选"`
}

type OrderExportRes struct{}
