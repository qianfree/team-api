package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/refund"
	stripewebhook "github.com/stripe/stripe-go/v81/webhook"

	"github.com/qianfree/team-api/internal/consts"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// StripeProvider Stripe 支付渠道实现。
type StripeProvider struct{}

func (p *StripeProvider) CreatePayment(ctx context.Context, order *PaymentOrder, config interface{}) (*PaymentResult, error) {
	cfg, ok := config.(*StripeConfig)
	if !ok {
		return nil, lcommon.NewBusinessError(consts.CodePaymentInvalidConfig, consts.MsgPaymentInvalidConfig)
	}
	if cfg.APISecret == "" {
		return nil, lcommon.NewBusinessError(consts.CodePaymentInvalidConfig, "Stripe API 密钥未配置")
	}

	stripe.Key = cfg.APISecret

	// 构建 Checkout Session 参数
	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(order.OrderNo),
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(order.ReturnURL + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:         stripe.String(order.ReturnURL + "?cancelled=true"),
		Metadata: map[string]string{
			"tenant_id":  gconv.String(order.TenantID),
			"order_id":   gconv.String(order.OrderID),
			"order_no":   order.OrderNo,
			"order_type": order.OrderType,
		},
	}

	// 使用 PriceID 或自定义单价
	if cfg.PriceID != "" {
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(cfg.PriceID),
				Quantity: stripe.Int64(int64(order.Amount)),
			},
		}
	} else {
		unitPrice := cfg.UnitPrice
		if unitPrice <= 0 {
			unitPrice = 1.0
		}
		// Stripe 金额以分为单位
		amountInCents := int64(order.Amount * unitPrice * 100)
		params.LineItems = []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(strings.ToLower(order.Currency)),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(order.Description),
					},
					UnitAmount: stripe.Int64(amountInCents),
				},
				Quantity: stripe.Int64(1),
			},
		}
	}

	s, err := session.New(params)
	if err != nil {
		return nil, gerror.Wrapf(err, "创建 Stripe Checkout Session 失败")
	}

	return &PaymentResult{
		PaymentURL: s.URL,
		PaymentNo:  s.ID,
		Params: map[string]string{
			"session_id":     s.ID,
			"payment_intent": s.PaymentIntent.ID,
		},
		IsRedirect: true,
	}, nil
}

func (p *StripeProvider) HandleCallback(ctx context.Context, r *http.Request, config interface{}) (*CallbackResult, error) {
	cfg, ok := config.(*StripeConfig)
	if !ok {
		return nil, lcommon.NewBusinessError(consts.CodePaymentInvalidConfig, consts.MsgPaymentInvalidConfig)
	}
	if cfg.WebhookSecret == "" {
		return nil, lcommon.NewBusinessError(consts.CodePaymentInvalidConfig, "Stripe Webhook 密钥未配置")
	}

	body, err := ReadBody(r)
	if err != nil {
		return nil, gerror.Wrapf(err, "读取请求体失败")
	}

	signature := r.Header.Get("Stripe-Signature")
	event, err := stripewebhook.ConstructEventWithOptions(body, signature, cfg.WebhookSecret, stripewebhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		return nil, gerror.Wrapf(err, "Stripe Webhook 签名验证失败")
	}

	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		return p.handleSessionCompleted(event)
	case stripe.EventTypeCheckoutSessionExpired:
		return p.handleSessionExpired(event)
	default:
		return nil, lcommon.NewBusinessError(consts.CodePaymentCallbackFailed, fmt.Sprintf("不支持的 Stripe 事件类型: %s", event.Type))
	}
}

func (p *StripeProvider) handleSessionCompleted(event stripe.Event) (*CallbackResult, error) {
	var cs stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
		return nil, gerror.Wrapf(err, "解析 CheckoutSession 数据失败")
	}

	if cs.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid {
		return &CallbackResult{
			OrderNo:     cs.ClientReferenceID,
			TradeNo:     cs.ID,
			TradeStatus: string(cs.PaymentStatus),
			Success:     false,
			RawData:     string(event.Data.Raw),
		}, nil
	}

	// 解析金额（Stripe 以分为单位）
	var paidAmount float64
	if cs.AmountTotal > 0 {
		paidAmount = float64(cs.AmountTotal) / 100.0
	}

	return &CallbackResult{
		OrderNo:     cs.ClientReferenceID,
		TradeNo:     cs.ID,
		TradeStatus: "TRADE_SUCCESS",
		PaidAmount:  paidAmount,
		Success:     true,
		RawData:     string(event.Data.Raw),
	}, nil
}

func (p *StripeProvider) handleSessionExpired(event stripe.Event) (*CallbackResult, error) {
	var cs stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &cs); err != nil {
		return nil, gerror.Wrapf(err, "解析 CheckoutSession 数据失败")
	}

	return &CallbackResult{
		OrderNo:     cs.ClientReferenceID,
		TradeNo:     cs.ID,
		TradeStatus: "expired",
		Success:     false,
		RawData:     string(event.Data.Raw),
	}, nil
}

func (p *StripeProvider) Refund(ctx context.Context, refundReq *RefundRequest, config interface{}) error {
	cfg, ok := config.(*StripeConfig)
	if !ok {
		return lcommon.NewBusinessError(consts.CodePaymentInvalidConfig, consts.MsgPaymentInvalidConfig)
	}
	if cfg.APISecret == "" {
		return lcommon.NewBusinessError(consts.CodePaymentInvalidConfig, "Stripe API 密钥未配置")
	}

	stripe.Key = cfg.APISecret

	// 通过 Checkout Session ID 获取 PaymentIntent，然后发起退款
	params := &stripe.RefundParams{
		PaymentIntent: stripe.String(refundReq.PaymentNo),
		Amount:        stripe.Int64(int64(refundReq.RefundAmount * 100)), // 转为分
		Reason:        stripe.String(string(stripe.RefundReasonRequestedByCustomer)),
		Metadata: map[string]string{
			"order_id":      gconv.String(refundReq.OrderID),
			"order_no":      refundReq.OrderNo,
			"refund_reason": refundReq.Reason,
		},
	}

	_, err := refund.New(params)
	if err != nil {
		return gerror.Wrapf(err, "Stripe 退款失败")
	}

	return nil
}

func (p *StripeProvider) QueryPaymentStatus(ctx context.Context, paymentNo string, config interface{}) (*PaymentStatus, error) {
	cfg, ok := config.(*StripeConfig)
	if !ok {
		return nil, lcommon.NewBusinessError(consts.CodePaymentInvalidConfig, consts.MsgPaymentInvalidConfig)
	}

	stripe.Key = cfg.APISecret

	s, err := session.Get(paymentNo, nil)
	if err != nil {
		return nil, gerror.Wrapf(err, "查询 Stripe Session 失败")
	}

	status := "pending"
	var paidAmount float64
	if s.PaymentStatus == stripe.CheckoutSessionPaymentStatusPaid {
		status = "paid"
		if s.AmountTotal > 0 {
			paidAmount = float64(s.AmountTotal) / 100.0
		}
	} else if s.Status == stripe.CheckoutSessionStatusExpired {
		status = "expired"
	}

	return &PaymentStatus{
		TradeNo:    s.ID,
		Status:     status,
		PaidAmount: paidAmount,
	}, nil
}
