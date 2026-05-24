package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

// === 租户订单 ===

type TenantOrderCreateReq struct {
	g.Meta    `path:"/orders/create" method:"post" mime:"json" tags:"租户控制台-订单" summary:"创建订单"`
	PlanID    int64 `json:"plan_id" v:"required|min:1"`
	Months    int   `json:"months"`
	AutoRenew bool  `json:"auto_renew"`
}

type TenantOrderCreateRes struct {
	Data map[string]any `json:"data"`
}

type TenantOrderPayReq struct {
	g.Meta         `path:"/orders/{id}/pay" method:"post" mime:"json" tags:"租户控制台-订单" summary:"支付订单"`
	Id             int64  `json:"id" in:"path" v:"required|min:1"`
	PaymentChannel string `json:"payment_channel" dc:"支付渠道类型（epay/stripe/mock）"`
	PaymentMethod  string `json:"payment_method"`
}

type TenantOrderPayRes struct {
	Data map[string]any `json:"data"`
}

type TenantOrderListReq struct {
	g.Meta   `path:"/orders" method:"get" mime:"json" tags:"租户控制台-订单" summary:"订单列表"`
	Page     int    `json:"page" in:"query" d:"1"`
	PageSize int    `json:"page_size" in:"query" d:"20"`
	Status   string `json:"status" in:"query"`
}

type TenantOrderItem struct {
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

type TenantOrderListRes struct {
	List     []*TenantOrderItem `json:"list"`
	Total    int                `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

type TenantOrderDetailReq struct {
	g.Meta `path:"/orders/{id}" method:"get" mime:"json" tags:"租户控制台-订单" summary:"订单详情"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantOrderDetailRes struct {
	Data map[string]any `json:"data"`
}

type TenantOrderCancelReq struct {
	g.Meta `path:"/orders/{id}/cancel" method:"post" mime:"json" tags:"租户控制台-订单" summary:"取消订单"`
	Id     int64 `json:"id" in:"path" v:"required|min:1"`
}

type TenantOrderCancelRes struct{}

type TenantRechargeCreateReq struct {
	g.Meta         `path:"/recharge/create" method:"post" mime:"json" tags:"租户控制台-订单" summary:"创建充值订单"`
	Amount         float64 `json:"amount" v:"required|min:0.01#请输入充值金额|充值金额不能小于 0.01"`
	PaymentChannel string  `json:"payment_channel" v:"required|in:epay#请选择支付渠道|不支持的支付渠道"`
	PaymentMethod  string  `json:"payment_method" v:"required#请选择支付方式"`
}

type TenantRechargeCreateRes struct {
	OrderId     int64             `json:"order_id"`
	OrderNo     string            `json:"order_no"`
	PaymentUrl  string            `json:"payment_url"`
	PaymentNo   string            `json:"payment_no"`
	Params      map[string]string `json:"params,omitempty"`
	IsRedirect  bool              `json:"is_redirect"`
	FinalAmount float64           `json:"final_amount"`
}

type TenantPaymentInfoReq struct {
	g.Meta `path:"/payment-info" method:"get" mime:"json" tags:"租户控制台-订单" summary:"支付信息"`
}

type TenantPaymentInfoRes struct {
	Channels       []map[string]any `json:"channels"`
	AmountOptions  []int            `json:"amount_options,omitempty"`
	AmountDiscount map[int]float64  `json:"amount_discount,omitempty"`
	MinTopUp       float64          `json:"min_topup,omitempty"`
	Currency       string           `json:"currency,omitempty"`
}

// TenantOrderExportReq 导出订单列表请求
type TenantOrderExportReq struct {
	g.Meta `path:"/orders/export" method:"get" mime:"json" tags:"租户控制台-订单" summary:"导出订单列表"`
	Format string `json:"format" in:"query" d:"csv" v:"in:csv,xlsx" dc:"导出格式：csv / xlsx"`
	Status string `json:"status" in:"query"`
}

type TenantOrderExportRes struct{}
