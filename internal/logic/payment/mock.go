package payment

import (
	"context"
	do "github.com/qianfree/team-api/internal/model/do"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
)

// MockProvider Mock 支付渠道（开发环境）。
type MockProvider struct{}

func (p *MockProvider) CreatePayment(ctx context.Context, order *PaymentOrder, config interface{}) (*PaymentResult, error) {
	return &PaymentResult{
		PaymentURL: "",
		PaymentNo:  "MOCK_" + order.OrderNo,
		Params:     map[string]string{"mock": "true"},
		IsRedirect: false,
	}, nil
}

func (p *MockProvider) HandleCallback(ctx context.Context, r *http.Request, config interface{}) (*CallbackResult, error) {
	return &CallbackResult{
		OrderNo:     r.URL.Query().Get("order_no"),
		TradeNo:     "MOCK_CALLBACK",
		TradeStatus: "TRADE_SUCCESS",
		Success:     true,
	}, nil
}

func (p *MockProvider) Refund(ctx context.Context, refund *RefundRequest, config interface{}) error {
	return nil
}

func (p *MockProvider) QueryPaymentStatus(ctx context.Context, paymentNo string, config interface{}) (*PaymentStatus, error) {
	return &PaymentStatus{
		TradeNo: paymentNo,
		Status:  "paid",
	}, nil
}

// MockPay 模拟支付（开发环境），向后兼容。
func MockPay(ctx context.Context, orderID int64) error {
	var status string
	err := dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Fields("status").
		Scan(&status)
	if err != nil {
		return err
	}
	if status != "pending" {
		return nil
	}

	_, err = dao.OrdOrders.Ctx(ctx).
		Where("id", orderID).
		Where("status", "pending").
		Data(do.OrdOrders{
			Status:    "paid",
			PaidAt:    gtime.Now(),
			PaymentNo: "MOCK_" + g.Cfg().MustGet(ctx, "server.name").String(),
		}).Update()
	if err != nil {
		return err
	}

	return FulfillOrder(ctx, orderID)
}
