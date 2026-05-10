package payment

import (
	"context"
	"encoding/json"

	"github.com/qianfree/team-api/internal/logic/common"
)

// GetGlobalPaymentSettings loads global payment settings from ConfigService.
func GetGlobalPaymentSettings(ctx context.Context) (*GlobalPaymentSettings, error) {
	cfg := common.Config()
	settings := &GlobalPaymentSettings{
		AmountOptions:  []int{10, 20, 50, 100, 200, 500},
		AmountDiscount: map[int]float64{},
		MinTopUp:       1,
		Currency:       "CNY",
	}

	if val := cfg.GetString(ctx, "payment_amount_options"); val != "" {
		json.Unmarshal([]byte(val), &settings.AmountOptions)
	}
	if val := cfg.GetString(ctx, "payment_amount_discount"); val != "" {
		json.Unmarshal([]byte(val), &settings.AmountDiscount)
	}
	if n := cfg.GetInt(ctx, "payment_min_topup"); n > 0 {
		settings.MinTopUp = n
	}
	if val := cfg.GetString(ctx, "payment_currency"); val != "" {
		settings.Currency = val
	}
	settings.CallbackBaseURL = cfg.GetString(ctx, "payment_callback_base_url")

	return settings, nil
}

// SaveGlobalPaymentSettings persists global payment settings via ConfigService.
func SaveGlobalPaymentSettings(ctx context.Context, settings *GlobalPaymentSettings) error {
	cfg := common.Config()
	items := map[string]string{
		"payment_amount_options":    mustJSON(settings.AmountOptions),
		"payment_amount_discount":   mustJSON(settings.AmountDiscount),
		"payment_min_topup":         mustJSON(settings.MinTopUp),
		"payment_currency":          settings.Currency,
		"payment_callback_base_url": settings.CallbackBaseURL,
	}
	for key, value := range items {
		if err := cfg.SetOption(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

// GetCallbackBaseURL returns the payment callback base URL.
func GetCallbackBaseURL(ctx context.Context) string {
	return common.Config().GetString(ctx, "payment_callback_base_url")
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
