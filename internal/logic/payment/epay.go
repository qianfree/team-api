package payment

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// EpayProvider 易支付渠道实现。
// 直接实现易支付协议（MD5签名），不依赖第三方 SDK。
type EpayProvider struct{}

func (p *EpayProvider) CreatePayment(ctx context.Context, order *PaymentOrder, config interface{}) (*PaymentResult, error) {
	cfg, ok := config.(*EpayConfig)
	if !ok {
		return nil, lcommon.NewBadRequestError("无效的易支付配置")
	}
	if cfg.PayAddress == "" || cfg.MerchantID == "" || cfg.MerchantKey == "" {
		return nil, lcommon.NewBadRequestError("易支付配置不完整")
	}

	params := map[string]string{
		"pid":          cfg.MerchantID,
		"type":         order.PaymentMethod,
		"out_trade_no": order.OrderNo,
		"notify_url":   order.NotifyURL,
		"return_url":   order.ReturnURL,
		"name":         order.Description,
		"money":        fmt.Sprintf("%.2f", order.Amount),
		"device":       "pc",
	}

	sign := epaySign(params, cfg.MerchantKey)
	params["sign"] = sign
	params["sign_type"] = "MD5"

	redirectURL := fmt.Sprintf("%s/submit.php?%s", strings.TrimRight(cfg.PayAddress, "/"), buildQuery(params))

	return &PaymentResult{
		PaymentURL: redirectURL,
		Params:     params,
		IsRedirect: true,
	}, nil
}

func (p *EpayProvider) HandleCallback(ctx context.Context, r *http.Request, config interface{}) (*CallbackResult, error) {
	cfg, ok := config.(*EpayConfig)
	if !ok {
		return nil, lcommon.NewBadRequestError("无效的易支付配置")
	}

	if cfg.MerchantKey == "" {
		return nil, lcommon.NewBusinessError(422, "易支付商户密钥未配置，回调已拒绝")
	}

	// 同时支持 GET 和 POST 回调
	r.ParseForm()
	params := make(map[string]string)
	for k, v := range r.Form {
		if len(v) > 0 && k != "sign" && k != "sign_type" && v[0] != "" {
			params[k] = v[0]
		}
	}

	receivedSign := r.FormValue("sign")
	expectedSign := epaySign(params, cfg.MerchantKey)
	if receivedSign != expectedSign {
		return nil, lcommon.NewBusinessError(422, "易支付签名验证失败")
	}

	tradeStatus := r.FormValue("trade_status")
	var money float64
	fmt.Sscanf(r.FormValue("money"), "%f", &money)

	return &CallbackResult{
		OrderNo:     r.FormValue("out_trade_no"),
		TradeNo:     r.FormValue("trade_no"),
		TradeStatus: tradeStatus,
		PaidAmount:  money,
		Success:     tradeStatus == "TRADE_SUCCESS",
		RawData:     r.Form.Encode(),
	}, nil
}

func (p *EpayProvider) Refund(ctx context.Context, refund *RefundRequest, config interface{}) error {
	return lcommon.NewBusinessError(422, "易支付不支持 API 退款")
}

func (p *EpayProvider) QueryPaymentStatus(ctx context.Context, paymentNo string, config interface{}) (*PaymentStatus, error) {
	return nil, lcommon.NewBusinessError(422, "易支付暂不支持订单查询")
}

// epaySign 计算 MD5 签名：按 key 排序拼接参数 + 密钥，取 MD5。
func epaySign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		if params[k] == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(params[k])
	}
	buf.WriteString(key)

	hash := md5.Sum([]byte(buf.String()))
	return hex.EncodeToString(hash[:])
}

// buildQuery 构建 URL 查询字符串。
func buildQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(url.QueryEscape(k))
		buf.WriteByte('=')
		buf.WriteString(url.QueryEscape(params[k]))
	}
	return buf.String()
}

// ReadBody 读取 HTTP 请求体并返回内容（用于 Stripe Webhook 等需要 raw body 的场景）。
func ReadBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
