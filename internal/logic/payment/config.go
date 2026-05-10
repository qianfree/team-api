package payment

import "encoding/json"

// EpayConfig 易支付渠道配置。
type EpayConfig struct {
	PayAddress  string          `json:"pay_address"`  // 易支付网关地址
	MerchantID  string          `json:"merchant_id"`  // 商户 ID
	MerchantKey string          `json:"merchant_key"` // 商户密钥（用于签名）
	PayMethods  []EpayPayMethod `json:"pay_methods"`  // 可用支付方式列表
}

// EpayPayMethod 易支付子支付方式。
type EpayPayMethod struct {
	Name  string `json:"name"`  // 显示名称（如"支付宝"、"微信支付"）
	Type  string `json:"type"`  // 方式标识（alipay/wxpay/qqpay 等）
	Color string `json:"color"` // UI 颜色提示
}

// StripeConfig Stripe 渠道配置。
type StripeConfig struct {
	APISecret     string  `json:"api_secret"`     // Stripe 密钥（sk_... / rk_...）
	WebhookSecret string  `json:"webhook_secret"` // Webhook 签名密钥（whsec_...）
	PriceID       string  `json:"price_id"`       // Stripe Price ID（用于 Checkout Session）
	UnitPrice     float64 `json:"unit_price"`     // 每单位价格（USD），当 PriceID 为空时使用
	MinTopup      int     `json:"min_topup"`      // 最低充值数量
	TestMode      bool    `json:"test_mode"`      // 是否测试模式
}

// MockConfig Mock 支付渠道配置（开发环境）。
type MockConfig struct {
	AutoFulfill bool `json:"auto_fulfill"` // 是否自动履约
}

// GlobalPaymentSettings 全局支付设置。
type GlobalPaymentSettings struct {
	AmountOptions   []int           `json:"amount_options"`    // 预设充值金额选项
	AmountDiscount  map[int]float64 `json:"amount_discount"`   // 金额折扣映射，如 {100: 0.9} 表示充 100 享 9 折
	MinTopUp        int             `json:"min_topup"`         // 最低充值金额
	Currency        string          `json:"currency"`          // 默认货币代码
	CallbackBaseURL string          `json:"callback_base_url"` // 回调基础 URL（为空则使用请求 Host）
}

// ParseChannelConfig 按渠道类型将 JSONB 配置字符串解析为对应的类型化结构体。
func ParseChannelConfig(channelType string, configJSON string) (interface{}, error) {
	switch channelType {
	case "epay":
		cfg := &EpayConfig{}
		if err := json.Unmarshal([]byte(configJSON), cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	case "stripe":
		cfg := &StripeConfig{}
		if err := json.Unmarshal([]byte(configJSON), cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	case "mock":
		cfg := &MockConfig{}
		if err := json.Unmarshal([]byte(configJSON), cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	default:
		return nil, nil
	}
}
