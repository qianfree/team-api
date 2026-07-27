package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

// 已注册的渠道类型列表。
var channelTypes = []string{"epay"}

// enabledChannelsCache 缓存 GetEnabledChannels 的聚合结果。
// 底层 GetOption 虽有 10min 缓存，但每次调用仍要遍历所有渠道类型并逐个 JSON 解析、
// 过滤；租户支付信息页是高频读路径，这里加 60s 结果缓存摊薄开销。
// 配置变更时由 SaveChannelConfig 主动失效（P2-9）。
var enabledChannelsCache = lcommon.NewCache("pay_enabled_channels", 60*time.Second)

// enabledChannelsCacheKey 所有租户共享同一份启用渠道快照（渠道配置是平台级，非租户级），故使用固定 key。
const enabledChannelsCacheKey = "all"

// EpayConfig 易支付渠道配置。
type EpayConfig struct {
	IsEnabled   bool            `json:"is_enabled"`
	PayAddress  string          `json:"pay_address"`
	MerchantID  string          `json:"merchant_id"`
	MerchantKey string          `json:"merchant_key"`
	PayMethods  []EpayPayMethod `json:"pay_methods"`
}

// EpayPayMethod 易支付子支付方式。
type EpayPayMethod struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color"`
}

// GlobalPaymentSettings 全局支付设置。
type GlobalPaymentSettings struct {
	AmountOptions   []int           `json:"amount_options"`
	AmountDiscount  map[int]float64 `json:"amount_discount"`
	MinTopUp        float64         `json:"min_topup"`
	Currency        string          `json:"currency"`
	CallbackBaseURL string          `json:"callback_base_url"`
}

// channelConfigKey 返回渠道在 sys_options 中的存储键。
func channelConfigKey(channelType string) string {
	return "payment_channel_" + channelType
}

// LoadChannelConfig 从 sys_options 加载渠道配置，返回原始 JSON 字符串和解析后的配置。
func LoadChannelConfig(ctx context.Context, channelType string) (jsonStr string, cfg interface{}, err error) {
	jsonStr = lcommon.Config().GetOption(ctx, channelConfigKey(channelType))
	if jsonStr == "" {
		cfg = defaultChannelConfig(channelType)
		return
	}
	cfg, err = ParseChannelConfig(channelType, jsonStr)
	return
}

// SaveChannelConfig 将渠道配置 JSON 保存到 sys_options。
func SaveChannelConfig(ctx context.Context, channelType string, configJSON string) error {
	// 启用线上支付渠道前，必须先配置全局回调地址，否则网关异步通知无法回传，
	// 支付成功也无法入账（发起支付时还有 RequireCallbackBaseURL 二次校验）。
	if err := validateChannelEnable(ctx, channelType, configJSON); err != nil {
		return err
	}
	if err := lcommon.Config().SetOption(ctx, channelConfigKey(channelType), configJSON); err != nil {
		return err
	}
	// 渠道启用状态可能变化，失效已启用渠道快照缓存，下次查询重建。
	enabledChannelsCache.Delete(ctx, enabledChannelsCacheKey)
	return nil
}

// validateChannelEnable 校验"启用渠道"的前置条件：启用易支付要求回调地址已配置。
func validateChannelEnable(ctx context.Context, channelType string, configJSON string) error {
	if channelType != "epay" {
		return nil
	}
	cfg, err := ParseChannelConfig(channelType, configJSON)
	if err != nil || cfg == nil {
		return nil // JSON 合法性由调用方校验；此处只关心启用前置条件
	}
	ec, ok := cfg.(*EpayConfig)
	if !ok || !ec.IsEnabled {
		return nil
	}
	if strings.TrimSpace(GetCallbackBaseURL(ctx)) == "" {
		return lcommon.NewBadRequestError("请先在「支付设置」中配置回调基础URL，再启用支付渠道；否则支付成功后无法回调入账")
	}
	return nil
}

// RequireCallbackBaseURL 发起支付前的二次防护：回调地址未配置则拒绝发起。
// 渠道启用时已拦截一次，这里防止"启用后被清空回调地址"等场景下仍能发起支付。
func RequireCallbackBaseURL(ctx context.Context) error {
	if strings.TrimSpace(GetCallbackBaseURL(ctx)) == "" {
		return lcommon.NewBusinessError(422, "支付回调地址未配置，请联系管理员在「支付设置」中配置回调基础URL")
	}
	return nil
}

// ListAllChannels 返回所有已注册渠道的配置（含未配置的，返回默认值）。
func ListAllChannels(ctx context.Context) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(channelTypes))
	for _, ct := range channelTypes {
		_, cfg, _ := LoadChannelConfig(ctx, ct)
		item := map[string]interface{}{
			"channel": ct,
		}
		if c, ok := cfg.(*EpayConfig); ok {
			item["is_enabled"] = c.IsEnabled
			item["config"] = c
		}
		result = append(result, item)
	}
	return result
}

// GetEnabledChannels 返回所有已启用的渠道及其支付方式（供租户 PaymentInfo 使用）。
// 结果在进程内缓存 60s（渠道配置为平台级，非租户级），配置变更时由 SaveChannelConfig 失效。
func GetEnabledChannels(ctx context.Context) []map[string]interface{} {
	val, err := enabledChannelsCache.GetOrSet(ctx, enabledChannelsCacheKey, func(ctx context.Context) (any, error) {
		return loadEnabledChannels(ctx), nil
	})
	if err != nil || val == nil {
		return loadEnabledChannels(ctx)
	}
	if list, ok := val.([]map[string]interface{}); ok {
		return list
	}
	return loadEnabledChannels(ctx)
}

// loadEnabledChannels 遍历所有渠道类型，过滤出已启用者并组装支付方式。
func loadEnabledChannels(ctx context.Context) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	for _, ct := range channelTypes {
		jsonStr, cfg, err := LoadChannelConfig(ctx, ct)
		if err != nil {
			g.Log().Warningf(ctx, "[Payment] channel %s config parse error: %v", ct, err)
			continue
		}
		if cfg == nil {
			g.Log().Debugf(ctx, "[Payment] channel %s config is nil, jsonStr=%q", ct, jsonStr)
			continue
		}

		var isEnabled bool
		var payMethods []map[string]interface{}

		if c, ok := cfg.(*EpayConfig); ok {
			isEnabled = c.IsEnabled
			for _, m := range c.PayMethods {
				payMethods = append(payMethods, map[string]interface{}{
					"type":  m.Type,
					"name":  m.Name,
					"color": m.Color,
				})
			}
			g.Log().Debugf(ctx, "[Payment] channel %s: enabled=%v, pay_methods_count=%d, json_len=%d",
				ct, isEnabled, len(c.PayMethods), len(jsonStr))
		} else {
			g.Log().Warningf(ctx, "[Payment] channel %s config type mismatch: %T", ct, cfg)
		}

		if !isEnabled {
			continue
		}

		item := map[string]interface{}{
			"channel": ct,
		}
		if len(payMethods) > 0 {
			item["pay_methods"] = payMethods
		}
		result = append(result, item)
	}
	g.Log().Debugf(ctx, "[Payment] GetEnabledChannels returned %d channels", len(result))
	return result
}

// GetChannelConfigAndProvider 获取已启用渠道的配置和 Provider，未启用或不存在时返回错误。
func GetChannelConfigAndProvider(ctx context.Context, channelType string) (interface{}, error) {
	_, cfg, err := LoadChannelConfig(ctx, channelType)
	if err != nil {
		return nil, fmt.Errorf("支付渠道配置无效")
	}

	var isEnabled bool
	if c, ok := cfg.(*EpayConfig); ok {
		isEnabled = c.IsEnabled
	} else {
		return nil, fmt.Errorf("不支持的支付渠道: %s", channelType)
	}

	if !isEnabled {
		return nil, fmt.Errorf("支付渠道 %s 未启用", channelType)
	}

	// 校验关键配置完整性，防止空密钥导致签名可被伪造
	if c, ok := cfg.(*EpayConfig); ok {
		if strings.TrimSpace(c.PayAddress) == "" ||
			strings.TrimSpace(c.MerchantID) == "" ||
			strings.TrimSpace(c.MerchantKey) == "" {
			return nil, fmt.Errorf("支付渠道 %s 配置不完整", channelType)
		}
	}

	return cfg, nil
}

// ParseChannelConfig 按渠道类型将 JSON 字符串解析为对应的类型化结构体。
func ParseChannelConfig(channelType string, configJSON string) (interface{}, error) {
	switch channelType {
	case "epay":
		cfg := &EpayConfig{}
		if err := json.Unmarshal([]byte(configJSON), cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	default:
		return nil, nil
	}
}

// defaultChannelConfig 返回渠道类型的默认配置。
func defaultChannelConfig(channelType string) interface{} {
	switch channelType {
	case "epay":
		return &EpayConfig{PayMethods: []EpayPayMethod{}}
	default:
		return nil
	}
}
