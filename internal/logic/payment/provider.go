package payment

import (
	"context"
	"net/http"
)

// PaymentProvider 支付渠道统一接口，所有支付渠道必须实现。
type PaymentProvider interface {
	// CreatePayment 创建支付请求，返回支付跳转 URL 或支付参数。
	CreatePayment(ctx context.Context, order *PaymentOrder, config interface{}) (*PaymentResult, error)

	// HandleCallback 处理支付渠道的异步回调通知。
	HandleCallback(ctx context.Context, r *http.Request, config interface{}) (*CallbackResult, error)

	// Refund 发起退款。
	Refund(ctx context.Context, refund *RefundRequest, config interface{}) error

	// QueryPaymentStatus 查询支付状态。
	QueryPaymentStatus(ctx context.Context, paymentNo string, config interface{}) (*PaymentStatus, error)
}

// PaymentOrder 发起支付所需的订单数据。
type PaymentOrder struct {
	OrderID       int64   // 订单 ID
	OrderNo       string  // 订单号
	TenantID      int64   // 租户 ID
	Amount        float64 // 支付金额
	Currency      string  // 货币代码
	OrderType     string  // 订单类型（new_plan/renew/upgrade/recharge）
	Description   string  // 订单描述（用于支付页面展示）
	PaymentMethod string  // 子支付方式（alipay/wxpay 等）
	NotifyURL     string  // 异步回调地址
	ReturnURL     string  // 支付完成前端跳转地址
}

// PaymentResult 创建支付后返回的结果。
type PaymentResult struct {
	PaymentURL string            // 支付跳转 URL
	PaymentNo  string            // 渠道交易号（若创建时即返回）
	Params     map[string]string // 附加参数（透传给前端）
	IsRedirect bool              // 是否需要跳转到 PaymentURL
}

// CallbackResult 回调处理结果。
type CallbackResult struct {
	OrderNo     string  // 我方订单号
	TradeNo     string  // 渠道交易号
	TradeStatus string  // 渠道交易状态
	PaidAmount  float64 // 实付金额
	Success     bool    // 是否支付成功
	RawData     string  // 原始回调数据（审计用）
}

// RefundRequest 退款请求。
type RefundRequest struct {
	OrderID      int64
	OrderNo      string
	PaymentNo    string  // 原支付渠道交易号
	RefundAmount float64 // 退款金额
	Reason       string  // 退款原因
}

// PaymentStatus 支付状态查询结果。
type PaymentStatus struct {
	TradeNo    string  // 渠道交易号
	Status     string  // pending / paid / failed / expired
	PaidAmount float64 // 实付金额
	PaidAt     string  // 支付时间
}

// GetProvider 根据渠道类型返回对应的支付渠道实现。
func GetProvider(channelType string) PaymentProvider {
	switch channelType {
	case "epay":
		return &EpayProvider{}
	default:
		return nil
	}
}
