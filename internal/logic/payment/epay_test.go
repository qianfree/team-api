package payment

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func testEpayOrder() *PaymentOrder {
	return &PaymentOrder{
		OrderID:       1,
		OrderNo:       "ORD20261007000001",
		Amount:        10.5,
		Description:   "余额充值-10元", // 含中文，验证 GET 查询串编码
		PaymentMethod: "alipay",
		NotifyURL:     "https://api.example.com/api/payment/callback/epay",
		ReturnURL:     "https://console.example.com/wallet",
	}
}

func testEpayConfig(submitMethod string) *EpayConfig {
	return &EpayConfig{
		IsEnabled:    true,
		PayAddress:   "https://pay.example.com",
		MerchantID:   "1001",
		MerchantKey:  "testkey",
		SubmitMethod: submitMethod,
	}
}

// 默认（零值/空值）SubmitMethod 走 POST 表单：返回 action 地址 + Params
func TestEpayCreatePaymentDefaultPost(t *testing.T) {
	for _, method := range []string{"", "post", "POST"} {
		result, err := (&EpayProvider{}).CreatePayment(context.Background(), testEpayOrder(), testEpayConfig(method))
		if err != nil {
			t.Fatalf("SubmitMethod=%q: %v", method, err)
		}
		if result.PaymentURL != "https://pay.example.com/submit.php" {
			t.Fatalf("SubmitMethod=%q: PaymentURL = %q, want submit.php action 地址", method, result.PaymentURL)
		}
		if result.Params == nil || result.Params["sign"] == "" || result.Params["sign_type"] != "MD5" {
			t.Fatalf("SubmitMethod=%q: POST 模式应返回含 sign 的 Params", method)
		}
		if !result.IsRedirect {
			t.Fatalf("SubmitMethod=%q: IsRedirect 应为 true", method)
		}
	}
}

// GET 模式：返回完整跳转 URL，Params 为空；模拟网关侧「解码查询串 → 重算签名」
// 验签通过（签名对未编码原始值计算，编码只发生在传输层）
func TestEpayCreatePaymentGet(t *testing.T) {
	order := testEpayOrder()
	result, err := (&EpayProvider{}).CreatePayment(context.Background(), order, testEpayConfig("get"))
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if result.Params != nil {
		t.Fatalf("GET 模式不应返回 Params，got %v", result.Params)
	}
	if !strings.HasPrefix(result.PaymentURL, "https://pay.example.com/submit.php?") {
		t.Fatalf("GET 模式 PaymentURL 应为带查询串的完整地址, got %q", result.PaymentURL)
	}

	// 模拟网关：解析查询串（自动 percent-decode），剔除 sign/sign_type 后重算签名比对
	u, err := url.Parse(result.PaymentURL)
	if err != nil {
		t.Fatalf("PaymentURL 无法解析: %v", err)
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		t.Fatalf("查询串无法解析: %v", err)
	}
	if query.Get("name") != order.Description {
		t.Fatalf("解码后 name = %q, want %q（中文应正确编解码）", query.Get("name"), order.Description)
	}
	if query.Get("money") != "10.50" {
		t.Fatalf("money = %q, want 10.50", query.Get("money"))
	}

	received := map[string]string{}
	for k, v := range query {
		if len(v) > 0 && k != "sign" && k != "sign_type" {
			received[k] = v[0]
		}
	}
	if want := epaySign(received, "testkey"); want != query.Get("sign") {
		t.Fatalf("网关侧验签失败: sign = %q, want %q", query.Get("sign"), want)
	}
}

// POST 模式签名同样对原始值计算（与 GET 口径一致），不含 sign/sign_type 自身
func TestEpaySignRawValues(t *testing.T) {
	result, err := (&EpayProvider{}).CreatePayment(context.Background(), testEpayOrder(), testEpayConfig(""))
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	raw := map[string]string{}
	for k, v := range result.Params {
		if k != "sign" && k != "sign_type" {
			raw[k] = v
		}
	}
	if want := epaySign(raw, "testkey"); want != result.Params["sign"] {
		t.Fatalf("POST 模式签名口径不一致: sign = %q, want %q", result.Params["sign"], want)
	}
}
